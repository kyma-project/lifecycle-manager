package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/types"
)

func New(clnt client.Client) *RunnerImpl {
	return &RunnerImpl{
		Client:    clnt,
		versioner: schema.GroupVersions(clnt.Scheme().PreferredVersionAllGroups()),
		converter: clnt.Scheme(),
	}
}

type RunnerImpl struct {
	client.Client
	versioner runtime.GroupVersioner
	converter runtime.ObjectConvertor
}

// ReconcileManifests implements Runner.Sync.
func (r *RunnerImpl) ReconcileManifests(ctx context.Context, kyma *v1beta2.Kyma,
	modules common.Modules,
) error {
	ssaStart := time.Now()
	baseLogger := ctrlLog.FromContext(ctx)

	results := make(chan error, len(modules))
	for _, module := range modules {
		go func(module *common.Module) {
			// Due to module template visibility change, some module previously deployed should be removed.
			if errors.Is(module.Template.Err, channel.ErrTemplateNotAllowed) {
				results <- r.deleteManifest(ctx, module)
				return
			}
			// Module template in other error status should be ignored.
			if module.Template.Err != nil {
				results <- nil
				return
			}
			if err := r.updateManifests(ctx, kyma, module); err != nil {
				results <- fmt.Errorf("could not update module %s: %w", module.GetName(), err)
				return
			}
			module.Logger(baseLogger).V(log.DebugLevel).Info("successfully patched module")
			results <- nil
		}(module)
	}
	var errs []error
	for i := 0; i < len(modules); i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}
	ssaFinish := time.Since(ssaStart)
	if len(errs) != 0 {
		return fmt.Errorf("ServerSideApply failed (after %s): %w", ssaFinish, types.NewMultiError(errs))
	}
	baseLogger.V(log.DebugLevel).Info("ServerSideApply finished", "time", ssaFinish)
	return nil
}

func (r *RunnerImpl) getModule(ctx context.Context, module client.Object) error {
	return r.Get(ctx, client.ObjectKey{Namespace: module.GetNamespace(), Name: module.GetName()}, module)
}

func (r *RunnerImpl) updateManifests(ctx context.Context, kyma *v1beta2.Kyma,
	module *common.Module,
) error {
	if err := r.setupModule(module, kyma); err != nil {
		return err
	}
	obj, err := r.converter.ConvertToVersion(module.Object, r.versioner)
	if err != nil {
		return err
	}
	manifestObj := obj.(client.Object)
	if err := r.Patch(ctx, manifestObj,
		client.Apply,
		client.FieldOwner(kyma.Labels[v1beta2.ManagedBy]),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("error applying manifest %s: %w", client.ObjectKeyFromObject(module), err)
	}
	module.Object = manifestObj

	return nil
}

func (r *RunnerImpl) deleteManifest(ctx context.Context, module *common.Module) error {
	err := r.Delete(ctx, module.Object)
	if apiErrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *RunnerImpl) setupModule(module *common.Module, kyma *v1beta2.Kyma) error {
	// set labels
	module.ApplyLabelsAndAnnotations(kyma)
	refs := module.GetOwnerReferences()
	if len(refs) == 0 {
		// set owner reference
		if err := controllerutil.SetControllerReference(kyma, module.Object, r.Scheme()); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
				module.GetName(), kyma.Name, err)
		}
	}

	return nil
}

func (r *RunnerImpl) SyncModuleStatus(ctx context.Context, kyma *v1beta2.Kyma, modules common.Modules) {
	r.updateModuleStatusFromExistingModules(modules, kyma)
	DeleteNoLongerExistingModuleStatus(ctx, kyma, r.getModule)
}

func (r *RunnerImpl) updateModuleStatusFromExistingModules(
	modules common.Modules,
	kyma *v1beta2.Kyma,
) {
	moduleStatusMap := kyma.GetModuleStatusMap()

	for idx := range modules {
		module := modules[idx]
		latestModuleStatus := generateModuleStatus(module)
		moduleStatus, exists := moduleStatusMap[module.ModuleName]
		if exists {
			*moduleStatus = latestModuleStatus
		} else {
			kyma.Status.Modules = append(kyma.Status.Modules, latestModuleStatus)
		}
	}
}

func generateModuleStatus(module *common.Module) v1beta2.ModuleStatus {
	if module.Template.Err != nil {
		return v1beta2.ModuleStatus{
			Name:    module.ModuleName,
			Channel: module.Template.DesiredChannel,
			FQDN:    module.FQDN,
			State:   v1beta2.StateError,
			Message: module.Template.Err.Error(),
		}
	}

	manifestAPIVersion, manifestKind := module.Object.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	templateAPIVersion, templateKind := module.Template.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return v1beta2.ModuleStatus{
		Name:    module.ModuleName,
		FQDN:    module.FQDN,
		State:   stateFromManifest(module.Object),
		Channel: module.Template.Spec.Channel,
		Version: module.Version,
		Manifest: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMetaFromObject(module.Object),
			TypeMeta:    metav1.TypeMeta{Kind: manifestKind, APIVersion: manifestAPIVersion},
		},
		Template: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMetaFromObject(module.Template),
			TypeMeta:    metav1.TypeMeta{Kind: templateKind, APIVersion: templateAPIVersion},
		},
	}
}

func stateFromManifest(obj client.Object) v1beta2.State {
	switch manifest := obj.(type) {
	case *v1beta2.Manifest:
		return v1beta2.State(manifest.Status.State)
	case *unstructured.Unstructured:
		state, _, _ := unstructured.NestedString(manifest.Object, "status", "state")
		return v1beta2.State(state)
	default:
		return ""
	}
}

func DeleteNoLongerExistingModuleStatus(
	ctx context.Context,
	kyma *v1beta2.Kyma,
	moduleFunc GetModuleFunc,
) {
	moduleStatusMap := kyma.GetModuleStatusMap()
	moduleStatus := kyma.GetNoLongerExistingModuleStatus()
	for idx := range moduleStatus {
		moduleStatus := moduleStatus[idx]
		if moduleStatus.Manifest == nil {
			delete(moduleStatusMap, moduleStatus.Name)
			continue
		}
		module := &unstructured.Unstructured{}
		module.SetGroupVersionKind(moduleStatus.Manifest.GroupVersionKind())
		module.SetName(moduleStatus.Manifest.GetName())
		module.SetNamespace(moduleStatus.Manifest.GetNamespace())
		err := moduleFunc(ctx, module)
		if apiErrors.IsNotFound(err) {
			delete(moduleStatusMap, moduleStatus.Name)
		} else {
			moduleStatus.State = stateFromManifest(module)
		}
	}
	kyma.Status.Modules = convertToNewModuleStatus(moduleStatusMap)
}

func convertToNewModuleStatus(moduleStatusMap map[string]*v1beta2.ModuleStatus) []v1beta2.ModuleStatus {
	newModuleStatus := make([]v1beta2.ModuleStatus, 0)
	for _, moduleStatus := range moduleStatusMap {
		newModuleStatus = append(newModuleStatus, *moduleStatus)
	}
	return newModuleStatus
}
