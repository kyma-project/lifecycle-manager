package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrServerSideApplyFailed = errors.New("ServerSideApply failed")

func New(clnt client.Client) *Runner {
	return &Runner{
		Client:    clnt,
		versioner: schema.GroupVersions(clnt.Scheme().PreferredVersionAllGroups()),
		converter: clnt.Scheme(),
	}
}

type Runner struct {
	client.Client

	versioner machineryruntime.GroupVersioner
	converter machineryruntime.ObjectConvertor
}

func (r *Runner) ReconcileManifests(ctx context.Context, kyma *v1beta2.Kyma,
	modules modulecommon.Modules,
) error {
	ssaStart := time.Now()
	baseLogger := logf.FromContext(ctx)

	results := make(chan error, len(modules))
	for _, module := range modules {
		go func(module *modulecommon.Module) {
			// Should not happen, but in case of NPE, we should stop process further.
			if module.TemplateInfo == nil {
				results <- nil
				return
			}
			// Due to module template visibility change, some module previously deployed should be removed.
			if errors.Is(module.TemplateInfo.Err, templatelookup.ErrTemplateNotAllowed) {
				results <- r.deleteManifest(ctx, module)
				return
			}
			// ModuleInStatus template in other error status should be ignored.
			if module.TemplateInfo.Err != nil {
				results <- nil
				return
			}
			if err := r.updateManifest(ctx, kyma, module); err != nil {
				results <- fmt.Errorf("could not update module %s: %w", module.Manifest.GetName(), err)
				return
			}
			module.Logger(baseLogger).V(log.DebugLevel).Info("successfully patched module")
			results <- nil
		}(module)
	}
	var errs []error
	for range modules {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}
	ssaFinish := time.Since(ssaStart)
	if len(errs) != 0 {
		errs = append(errs, fmt.Errorf("%w (after %s)", ErrServerSideApplyFailed, ssaFinish))
		return errors.Join(errs...)
	}
	baseLogger.V(log.DebugLevel).Info("ServerSideApply finished", "time", ssaFinish)
	return nil
}

func (r *Runner) updateManifest(ctx context.Context, kyma *v1beta2.Kyma,
	module *modulecommon.Module,
) error {
	if err := r.setupModule(module, kyma); err != nil {
		return err
	}
	obj, err := r.converter.ConvertToVersion(module.Manifest, r.versioner)
	if err != nil {
		return fmt.Errorf("failed to convert object to version: %w", err)
	}
	newManifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return common.ErrTypeAssert
	}

	moduleStatus := kyma.GetModuleStatusMap()[module.ModuleName]
	manifestInCluster, err := r.getManifest(ctx, newManifest.GetName(), newManifest.GetNamespace())
	if err != nil {
		return err
	}

	if err := r.doUpdateWithStrategy(ctx, kyma.Labels[shared.ManagedBy], module,
		manifestInCluster, newManifest, moduleStatus); err != nil {
		return err
	}
	module.Manifest = newManifest
	module.Manifest.Status = getManifestStatus(newManifest, manifestInCluster)
	return nil
}

func getManifestStatus(manifest, manifestInCluster *v1beta2.Manifest) shared.Status {
	// In case manifest in cluster exists, collect status from it.
	if manifestInCluster != nil {
		return manifestInCluster.Status
	}
	// status could also come from manifest after patch.
	return manifest.Status
}

func (r *Runner) doUpdateWithStrategy(ctx context.Context, owner string, module *modulecommon.Module,
	manifestInCluster, newManifest *v1beta2.Manifest, kymaModuleStatus *v1beta2.ModuleStatus,
) error {
	if !NeedToUpdate(manifestInCluster, newManifest, kymaModuleStatus, module) {
		return nil
	}
	if module.Enabled {
		return r.patchManifest(ctx, owner, newManifest)
	}
	// For disabled module, the manifest CR is under deleting, in this case,
	// we only update the spec when it's still not deleted.
	if err := r.updateAvailableManifestSpec(ctx, manifestInCluster, newManifest); err != nil && !util.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *Runner) getManifest(ctx context.Context, name, namespace string) (*v1beta2.Manifest, error) {
	manifestInCluster := &v1beta2.Manifest{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, manifestInCluster)
	if err != nil {
		if util.IsNotFound(err) {
			return nil, nil //nolint:nilnil //use nil to indicate an empty Manifest
		}
		return nil, fmt.Errorf("error get manifest %s/%s: %w", namespace, name,
			err)
	}

	return manifestInCluster, nil
}

func (r *Runner) patchManifest(ctx context.Context, owner string, newManifest *v1beta2.Manifest) error {
	if err := r.Patch(ctx, newManifest,
		client.Apply,
		client.FieldOwner(owner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("error applying manifest %s: %w", client.ObjectKeyFromObject(newManifest), err)
	}
	return nil
}

func (r *Runner) updateAvailableManifestSpec(ctx context.Context,
	manifestInCluster, newManifest *v1beta2.Manifest,
) error {
	if manifestInCluster == nil {
		return nil
	}
	manifestInCluster.Spec = newManifest.Spec
	if err := r.Update(ctx, manifestInCluster); err != nil {
		return fmt.Errorf("error update manifest %s: %w", client.ObjectKeyFromObject(newManifest), err)
	}
	return nil
}

func NeedToUpdate(manifestInCluster, newManifest *v1beta2.Manifest, moduleInStatus *v1beta2.ModuleStatus,
	module *modulecommon.Module,
) bool {
	if manifestInCluster == nil {
		return !(module.IsUnmanaged)
	}

	if manifestInCluster.IsUnmanaged() {
		return false
	}

	if module.IsUnmanaged {
		return true
	}

	diffInSpec := newManifest.Spec.Version != manifestInCluster.Spec.Version ||
		!newManifest.IsSameChannel(manifestInCluster)
	if manifestInCluster.IsMandatoryModule() || moduleInStatus == nil {
		return diffInSpec
	}

	diffInTemplate := moduleInStatus.Template != nil &&
		moduleInStatus.Template.GetGeneration() != module.TemplateInfo.GetGeneration()
	return diffInTemplate || diffInSpec
}

func (r *Runner) deleteManifest(ctx context.Context, module *modulecommon.Module) error {
	err := r.Delete(ctx, module.Manifest)
	if util.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("failed to delete manifest: %w", err)
}

func (r *Runner) setupModule(module *modulecommon.Module, kyma *v1beta2.Kyma) error {
	module.ApplyDefaultMetaToManifest(kyma)

	refs := module.Manifest.GetOwnerReferences()
	if len(refs) == 0 {
		if err := controllerutil.SetControllerReference(kyma, module.Manifest, r.Scheme()); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
				module.Manifest.GetName(), kyma.Name, err)
		}
	}

	return nil
}
