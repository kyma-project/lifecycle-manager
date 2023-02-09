package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"
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

// Sync implements Runner.Sync.
func (r *RunnerImpl) Sync(ctx context.Context, kyma *v1alpha1.Kyma,
	modules common.Modules,
) error {
	ssaStart := time.Now()
	baseLogger := ctrlLog.FromContext(ctx)

	results := make(chan error, len(modules))
	for _, module := range modules {
		go func(module *common.Module) {
			if err := r.updateModule(ctx, kyma, module); err != nil {
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

func (r *RunnerImpl) updateModule(ctx context.Context, kyma *v1alpha1.Kyma,
	module *common.Module,
) error {
	if err := r.setupModule(module, kyma); err != nil {
		return err
	}
	obj, err := r.converter.ConvertToVersion(module.Object, r.versioner)
	if err != nil {
		return err
	}
	clObj := obj.(client.Object)
	if err := r.Patch(ctx, clObj,
		client.Apply,
		client.FieldOwner(kyma.Labels[v1alpha1.ManagedBy]),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("error applying manifest %s: %w", client.ObjectKeyFromObject(module), err)
	}
	module.Object = clObj

	return nil
}

func (r *RunnerImpl) setupModule(module *common.Module, kyma *v1alpha1.Kyma) error {
	// set labels
	module.ApplyLabelsAndAnnotations(kyma)

	if module.GetOwnerReferences() == nil {
		// set owner reference
		if err := controllerutil.SetControllerReference(kyma, module.Object, r.Scheme()); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
				module.GetName(), kyma.Name, err)
		}
	}

	return nil
}

func (r *RunnerImpl) SyncModuleStatus(ctx context.Context, kyma *v1alpha1.Kyma, modules common.Modules) {
	r.updateModuleStatusFromExistingModules(modules, kyma)
	r.deleteNoLongerExistingModuleStatus(ctx, kyma)
}

func (r *RunnerImpl) updateModuleStatusFromExistingModules(modules common.Modules, kyma *v1alpha1.Kyma) {
	for idx := range modules {
		module := modules[idx]
		manifestAPIVersion, manifestKind := module.Object.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
		templateAPIVersion, templateKind := module.Template.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
		latestModuleStatus := v1alpha1.ModuleStatus{
			Name:    module.ModuleName,
			FQDN:    module.FQDN,
			State:   stateFromManifest(module.Object),
			Channel: module.Template.Spec.Channel,
			Version: module.Version,
			Manifest: v1alpha1.TrackingObject{
				PartialMeta: v1alpha1.PartialMetaFromObject(module.Object),
				TypeMeta:    metav1.TypeMeta{Kind: manifestKind, APIVersion: manifestAPIVersion},
			},
			Template: v1alpha1.TrackingObject{
				PartialMeta: v1alpha1.PartialMetaFromObject(module.Template),
				TypeMeta:    metav1.TypeMeta{Kind: templateKind, APIVersion: templateAPIVersion},
			},
		}
		if len(kyma.Status.Modules) < idx+1 {
			kyma.Status.Modules = append(kyma.Status.Modules, latestModuleStatus)
		} else {
			kyma.Status.Modules[idx] = latestModuleStatus
		}
	}
}

func stateFromManifest(obj client.Object) v1alpha1.State {
	switch manifest := obj.(type) {
	case *v1alpha1.Manifest:
		return v1alpha1.State(manifest.Status.State)
	default:
		return v1alpha1.StateError
	}
}

func (r *RunnerImpl) deleteNoLongerExistingModuleStatus(ctx context.Context, kyma *v1alpha1.Kyma) {
	moduleStatusArr := kyma.GetNoLongerExistingModuleStatus()
	for idx := range moduleStatusArr {
		moduleStatus := moduleStatusArr[idx]
		module := unstructured.Unstructured{}
		module.SetGroupVersionKind(moduleStatus.Manifest.GroupVersionKind())
		module.SetName(moduleStatus.Manifest.GetName())
		module.SetNamespace(moduleStatus.Manifest.GetNamespace())
		err := r.getModule(ctx, &module)
		if errors.IsNotFound(err) {
			kyma.Status.Modules = append(kyma.Status.Modules[:idx], kyma.Status.Modules[idx+1:]...)
		}
	}
}
