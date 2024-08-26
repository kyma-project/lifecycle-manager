package sync

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	commonerrs "github.com/kyma-project/lifecycle-manager/pkg/common" //nolint:importas // a one-time reference for the package
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
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

type ModuleMetrics interface {
	RemoveModuleStateMetrics(kymaName, moduleName string)
}

type GetModuleFunc func(ctx context.Context, module client.Object) error

type Runner struct {
	client.Client
	versioner machineryruntime.GroupVersioner
	converter machineryruntime.ObjectConvertor
}

func (r *Runner) ReconcileManifests(ctx context.Context, kyma *v1beta2.Kyma,
	modules common.Modules,
) error {
	ssaStart := time.Now()
	baseLogger := logf.FromContext(ctx)

	results := make(chan error, len(modules))
	for _, module := range modules {
		go func(module *common.Module) {
			// Should not happen, but in case of NPE, we should stop process further.
			if module.Template == nil {
				results <- nil
				return
			}
			// Due to module template visibility change, some module previously deployed should be removed.
			if errors.Is(module.Template.Err, templatelookup.ErrTemplateNotAllowed) {
				results <- r.deleteManifest(ctx, module)
				return
			}
			// ModuleInStatus template in other error status should be ignored.
			if module.Template.Err != nil {
				results <- nil
				return
			}
			if err := r.updateManifest(ctx, kyma, module); err != nil {
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
		errs = append(errs, fmt.Errorf("%w (after %s)", ErrServerSideApplyFailed, ssaFinish))
		return errors.Join(errs...)
	}
	baseLogger.V(log.DebugLevel).Info("ServerSideApply finished", "time", ssaFinish)
	return nil
}

func (r *Runner) getModule(ctx context.Context, module client.Object) error {
	err := r.Get(ctx, client.ObjectKey{Namespace: module.GetNamespace(), Name: module.GetName()}, module)
	if err != nil {
		return fmt.Errorf("failed to get module by name-namespace: %w", err)
	}
	return nil
}

func (r *Runner) updateManifest(ctx context.Context, kyma *v1beta2.Kyma,
	module *common.Module,
) error {
	logf.FromContext(ctx, "DEBUG", "KYMA").Info("update manifest")
	logf.FromContext(ctx, "DEBUG", "KYMA").Info("module IsUnmanaged", module.IsUnmanaged)
	if err := r.setupModule(module, kyma); err != nil {
		return err
	}
	logf.FromContext(ctx, "DEBUG", "KYMA").Info("setup module done")
	obj, err := r.converter.ConvertToVersion(module.Manifest, r.versioner)
	if err != nil {
		return fmt.Errorf("failed to convert object to version: %w", err)
	}
	manifestObj, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return commonerrs.ErrTypeAssert
	}

	moduleStatus := kyma.GetModuleStatusMap()[module.ModuleName]
	if err := r.patchOrUpdateManifest(ctx, kyma.Labels[shared.ManagedBy], module,
		manifestObj, moduleStatus); err != nil {
		return err
	}
	module.Manifest = manifestObj
	return nil
}

func (r *Runner) patchOrUpdateManifest(ctx context.Context,
	owner string,
	module *common.Module,
	manifestObj *v1beta2.Manifest,
	kymaModuleStatus *v1beta2.ModuleStatus,
) error {
	objKey := client.ObjectKeyFromObject(manifestObj)
	manifestInCluster := &v1beta2.Manifest{}
	if err := r.Get(ctx, objKey, manifestInCluster); err != nil {
		if !util.IsNotFound(err) {
			return fmt.Errorf("error getting manifest %s before update: %w", objKey, err)
		}
		manifestInCluster = nil
	}

	if manifestInCluster != nil && module.IsUnmanaged && manifestInCluster.IsUnmanaged() {
		return r.deleteManifest(ctx, module)
	}

	if !NeedToUpdate(manifestInCluster, manifestObj, kymaModuleStatus, module) {
		// Point to the current state from the cluster for the outside sync of the manifest
		manifestObj = manifestInCluster
		return nil
	}
	if module.Enabled {
		return r.patchManifest(ctx, owner, manifestObj)
	}
	// For disabled module, the manifest CR is under deleting, in this case, we only update the spec when it's still not deleted.
	if err := r.updateManifestForDisabledModule(ctx, manifestObj); err != nil && !util.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *Runner) patchManifest(ctx context.Context, owner string, manifestObj *v1beta2.Manifest) error {
	if err := r.Patch(ctx, manifestObj,
		client.Apply,
		client.FieldOwner(owner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("error applying manifest %s: %w", client.ObjectKeyFromObject(manifestObj), err)
	}
	return nil
}

func (r *Runner) updateManifestForDisabledModule(ctx context.Context, manifestObj *v1beta2.Manifest) error {
	manifestInCluster := &v1beta2.Manifest{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: manifestObj.GetNamespace(),
		Name:      manifestObj.GetName(),
	}, manifestInCluster); err != nil {
		return fmt.Errorf("error get manifest %s: %w", client.ObjectKeyFromObject(manifestObj), err)
	}
	manifestInCluster.Spec = manifestObj.Spec
	if err := r.Update(ctx, manifestInCluster); err != nil {
		return fmt.Errorf("error update manifest %s: %w", client.ObjectKeyFromObject(manifestObj), err)
	}
	return nil
}

func NeedToUpdate(manifestInCluster, manifestObj *v1beta2.Manifest, moduleStatus *v1beta2.ModuleStatus,
	module *common.Module,
) bool {
	if manifestInCluster == nil && module.IsUnmanaged {
		return false
	}
	if manifestInCluster == nil || moduleStatus == nil { // moduleStatus is nil in case of mandatory module
		return true
	}
	if moduleStatus.Template != nil && moduleStatus.Template.GetGeneration() != module.Template.GetGeneration() {
		return true
	}
	return manifestObj.Spec.Version != moduleStatus.Version ||
		manifestObj.Labels[shared.ChannelLabel] != moduleStatus.Channel ||
		moduleStatus.State != manifestInCluster.Status.State ||
		module.IsUnmanaged != manifestInCluster.IsUnmanaged()
}

func (r *Runner) deleteManifest(ctx context.Context, module *common.Module) error {
	err := r.Delete(ctx, module.Manifest)
	if util.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("failed to delete manifest: %w", err)
}

func (r *Runner) setupModule(module *common.Module, kyma *v1beta2.Kyma) error {
	module.ApplyDefaultMetaToManifest(kyma)

	refs := module.GetOwnerReferences()
	if len(refs) == 0 {
		if err := controllerutil.SetControllerReference(kyma, module.Manifest, r.Scheme()); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
				module.GetName(), kyma.Name, err)
		}
	}

	return nil
}

func (r *Runner) SyncModuleStatus(ctx context.Context, kyma *v1beta2.Kyma, modules common.Modules,
	metrics ModuleMetrics,
) {
	updateModuleStatusFromExistingModules(modules, kyma)
	debugLog := logf.FromContext(ctx).V(internal.DebugLogLevel).WithValues("DEBUG", "MODULES")
	debugLog.Info("SyncModulesStatus")
	for _, moduleStatus := range kyma.Status.Modules {
		debugLog.Info("ModuleStatus", "Name", moduleStatus.Name, "State", moduleStatus.State)
	}

	DeleteNoLongerExistingModuleStatus(ctx, kyma, r.getModule, metrics)
}

func updateModuleStatusFromExistingModules(
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
	if errors.Is(module.Template.Err, templatelookup.ErrTemplateUpdateNotAllowed) {
		newModuleStatus := existStatus.DeepCopy()
		newModuleStatus.State = shared.StateWarning
		newModuleStatus.Message = module.Template.Err.Error()
		return *newModuleStatus
	}
	if errors.Is(module.Template.Err, templatelookup.ErrNoTemplatesInListResult) {
		return v1beta2.ModuleStatus{
			Name:    module.ModuleName,
			Channel: module.Template.DesiredChannel,
			FQDN:    module.FQDN,
			State:   shared.StateWarning,
			Message: module.Template.Err.Error(),
		}
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
			PartialMeta: v1beta2.PartialMeta{
				Name:       manifestObject.Spec.Resource.GetName(),
				Namespace:  manifestObject.Spec.Resource.GetNamespace(),
				Generation: manifestObject.Spec.Resource.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: moduleCRKind, APIVersion: moduleCRAPIVersion},
		}

		if module.Template.Annotations[shared.IsClusterScopedAnnotation] == shared.EnableLabelValue {
			moduleResource.PartialMeta.Namespace = ""
		}
	}

	moduleStatus := v1beta2.ModuleStatus{
		Name:    module.ModuleName,
		FQDN:    module.FQDN,
		State:   manifestObject.Status.State,
		Channel: module.Template.Spec.Channel,
		Version: manifestObject.Spec.Version,
		// TODO remove manifest in case of unmanaged module
		Manifest: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       manifestObject.GetName(),
				Namespace:  manifestObject.GetNamespace(),
				Generation: manifestObject.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: manifestKind, APIVersion: manifestAPIVersion},
		},
		// TODO remove template in case of unmanaged module
		Template: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       module.Template.GetName(),
				Namespace:  module.Template.GetNamespace(),
				Generation: module.Template.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: templateKind, APIVersion: templateAPIVersion},
		},
		// TODO remove resource in case of unmanaged module
		Resource: moduleResource,
	}

	if module.IsUnmanaged {
		moduleStatus.State = shared.StateUnmanaged
	}

	return moduleStatus
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

func DeleteNoLongerExistingModuleStatus(ctx context.Context, kyma *v1beta2.Kyma, moduleFunc GetModuleFunc,
	metrics ModuleMetrics,
) {
	moduleStatusMap := kyma.GetModuleStatusMap()
	moduleStatusesToBeDeletedFromKymaStatus := kyma.GetNoLongerExistingModuleStatus()
	for idx := range moduleStatusesToBeDeletedFromKymaStatus {
		moduleStatus := moduleStatusesToBeDeletedFromKymaStatus[idx]
		if moduleStatus.Manifest == nil {
			if metrics != nil {
				metrics.RemoveModuleStateMetrics(kyma.Name, moduleStatus.Name)
			}
			delete(moduleStatusMap, moduleStatus.Name)
			continue
		}
		module := &unstructured.Unstructured{}
		module.SetGroupVersionKind(moduleStatus.Manifest.GroupVersionKind())
		module.SetName(moduleStatus.Manifest.GetName())
		module.SetNamespace(moduleStatus.Manifest.GetNamespace())
		err := moduleFunc(ctx, module)
		if util.IsNotFound(err) {
			if metrics != nil {
				metrics.RemoveModuleStateMetrics(kyma.Name, moduleStatus.Name)
			}
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
