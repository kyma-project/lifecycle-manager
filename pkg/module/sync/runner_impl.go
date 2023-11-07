package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma/metrics"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	commonErrors "github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
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
		errs = append(errs, fmt.Errorf("ServerSideApply failed (after %s)", ssaFinish)) //nolint:goerr113
		return errors.Join(errs...)
	}
	baseLogger.V(log.DebugLevel).Info("ServerSideApply finished", "time", ssaFinish)
	return nil
}

func (r *RunnerImpl) getModule(ctx context.Context, module client.Object) error {
	err := r.Get(ctx, client.ObjectKey{Namespace: module.GetNamespace(), Name: module.GetName()}, module)
	if err != nil {
		return fmt.Errorf("failed to get module by name-namespace: %w", err)
	}
	return nil
}

func (r *RunnerImpl) updateManifests(ctx context.Context, kyma *v1beta2.Kyma,
	module *common.Module,
) error {
	if err := r.setupModule(module, kyma); err != nil {
		return err
	}
	obj, err := r.converter.ConvertToVersion(module.Manifest, r.versioner)
	if err != nil {
		return fmt.Errorf("failed to convert object to version: %w", err)
	}
	manifestObj, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return commonErrors.ErrTypeAssert
	}
	if err := r.Patch(ctx, manifestObj,
		client.Apply,
		client.FieldOwner(kyma.Labels[v1beta2.ManagedBy]),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("error applying manifest %s: %w", client.ObjectKeyFromObject(module), err)
	}
	module.Manifest = manifestObj

	return nil
}

func (r *RunnerImpl) deleteManifest(ctx context.Context, module *common.Module) error {
	err := r.Delete(ctx, module.Manifest)
	if util.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("failed to delete manifest: %w", err)
}

func (r *RunnerImpl) setupModule(module *common.Module, kyma *v1beta2.Kyma) error {
	// set labels
	module.ApplyLabelsAndAnnotations(kyma)
	refs := module.GetOwnerReferences()
	if len(refs) == 0 {
		// set owner reference
		if err := controllerutil.SetControllerReference(kyma, module.Manifest, r.Scheme()); err != nil {
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
		moduleStatus, exists := moduleStatusMap[module.ModuleName]
		latestModuleStatus := generateModuleStatus(module, moduleStatus)
		if exists {
			*moduleStatus = latestModuleStatus
		} else {
			kyma.Status.Modules = append(kyma.Status.Modules, latestModuleStatus)
		}
	}
}

func generateModuleStatus(module *common.Module, existStatus *v1beta2.ModuleStatus) v1beta2.ModuleStatus {
	if errors.Is(module.Template.Err, channel.ErrTemplateUpdateNotAllowed) {
		newModuleStatus := existStatus.DeepCopy()
		newModuleStatus.State = shared.StateWarning
		newModuleStatus.Message = module.Template.Err.Error()
		return *newModuleStatus
	}
	if module.Template.Err != nil {
		return v1beta2.ModuleStatus{
			Name:    module.ModuleName,
			Channel: module.Template.DesiredChannel,
			FQDN:    module.FQDN,
			State:   shared.StateError,
			Message: module.Template.Err.Error(),
		}
	}
	manifestObject := module.Manifest
	manifestAPIVersion, manifestKind := manifestObject.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	templateAPIVersion, templateKind := module.Template.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	var moduleResource *v1beta2.TrackingObject
	if manifestObject.Spec.Resource != nil {
		moduleCRAPIVersion, moduleCRKind := manifestObject.Spec.Resource.
			GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
		moduleResource = &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMetaFromObject(manifestObject.Spec.Resource),
			TypeMeta:    metav1.TypeMeta{Kind: moduleCRKind, APIVersion: moduleCRAPIVersion},
		}

		if module.Template.Annotations[v1beta2.IsClusterScopedAnnotation] == v1beta2.EnableLabelValue {
			moduleResource.PartialMeta.Namespace = ""
		}
	}

	return v1beta2.ModuleStatus{
		Name:    module.ModuleName,
		FQDN:    module.FQDN,
		State:   manifestObject.Status.State,
		Channel: module.Template.Spec.Channel,
		Version: module.Version,
		Manifest: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMetaFromObject(manifestObject),
			TypeMeta:    metav1.TypeMeta{Kind: manifestKind, APIVersion: manifestAPIVersion},
		},
		Template: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMetaFromObject(module.Template),
			TypeMeta:    metav1.TypeMeta{Kind: templateKind, APIVersion: templateAPIVersion},
		},
		Resource: moduleResource,
	}
}

func stateFromManifest(obj client.Object) shared.State {
	switch manifest := obj.(type) {
	case *v1beta2.Manifest:
		return manifest.Status.State
	case *unstructured.Unstructured:
		state, _, _ := unstructured.NestedString(manifest.Object, "status", "state")
		return shared.State(state)
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
			metrics.RemoveModuleStateMetrics(kyma, moduleStatus.Name)
			delete(moduleStatusMap, moduleStatus.Name)
			continue
		}
		module := &unstructured.Unstructured{}
		module.SetGroupVersionKind(moduleStatus.Manifest.GroupVersionKind())
		module.SetName(moduleStatus.Manifest.GetName())
		module.SetNamespace(moduleStatus.Manifest.GetNamespace())
		err := moduleFunc(ctx, module)
		if util.IsNotFound(err) {
			metrics.RemoveModuleStateMetrics(kyma, moduleStatus.Name)
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
