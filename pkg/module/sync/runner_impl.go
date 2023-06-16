package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"github.com/tidwall/gjson"
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
		newModuleStatus.State = v1beta2.StateWarning
		newModuleStatus.Message = module.Template.Err.Error()
		return *newModuleStatus
	}
	if module.Template.Err != nil {
		status := v1beta2.ModuleStatus{
			Name:    module.ModuleName,
			Channel: module.Template.DesiredChannel,
			FQDN:    module.FQDN,
			State:   v1beta2.StateError,
			Message: module.Template.Err.Error(),
		}
		if module.Template.ModuleTemplate != nil {
			status.CustomStateCheck = module.Template.Spec.CustomStateCheck
		}
		return status
	}
	manifestObject, ok := module.Object.(*v1beta2.Manifest)
	if !ok {
		// TODO: impossible case, remove casting check after module use typed Manifest instead of client.Object
		return v1beta2.ModuleStatus{
			Name:             module.ModuleName,
			Channel:          module.Template.DesiredChannel,
			FQDN:             module.FQDN,
			State:            v1beta2.StateError,
			Message:          ErrManifestConversion.Error(),
			CustomStateCheck: module.Template.Spec.CustomStateCheck,
		}
	}
	return generateDefaultModuleStatus(module, manifestObject)
}

func generateDefaultModuleStatus(module *common.Module, manifestObject *v1beta2.Manifest) v1beta2.ModuleStatus {
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
	}

	return v1beta2.ModuleStatus{
		Name:    module.ModuleName,
		FQDN:    module.FQDN,
		State:   stateFromManifest(module.Object, module.Template.Spec.CustomStateCheck),
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
		Resource:         moduleResource,
		CustomStateCheck: module.Template.Spec.CustomStateCheck,
	}
}

func stateFromManifest(obj client.Object, customStateCheck *v1beta2.CustomStateCheck) v1beta2.State {
	if customStateCheck == nil {
		switch manifest := obj.(type) {
		case *v1beta2.Manifest:
			return v1beta2.State(manifest.Status.State)
		case *unstructured.Unstructured:
			state, _, _ := unstructured.NestedString(manifest.Object, "status", "state")
			return v1beta2.State(state)
		default:
			return ""
		}
	} else {
		return processCustomStateCheck(obj, customStateCheck)
	}
}

func processCustomStateCheck(obj client.Object, customStateCheck *v1beta2.CustomStateCheck) v1beta2.State {
	marshalledObj, err := json.Marshal(obj)
	if err != nil {
		return v1beta2.StateError
	}

	customStateCheck.JSONPath, _ = strings.CutPrefix(customStateCheck.JSONPath, ".")
	result := gjson.Get(string(marshalledObj), customStateCheck.JSONPath)

	if valueFromManifest, ok := result.Value().(string); ok && customStateCheck.Value == valueFromManifest {
		return v1beta2.StateReady
	} else if !result.Exists() {
		return v1beta2.StateError
	} else {
		return v1beta2.StateProcessing
	}
}

func DeleteNoLongerExistingModuleStatus(
	ctx context.Context,
	kyma *v1beta2.Kyma,
	moduleFunc GetModuleFunc,
) {
	logger := ctrlLog.FromContext(ctx).V(log.DebugLevel)
	moduleStatusMap := kyma.GetModuleStatusMap()
	moduleStatus := kyma.GetNoLongerExistingModuleStatus()
	for idx := range moduleStatus {
		moduleStatus := moduleStatus[idx]
		if moduleStatus.Manifest == nil {
			if err := metrics.RemoveModuleStateMetrics(kyma, moduleStatus.Name); err != nil {
				logger.Info(fmt.Sprintf("error occurred while removing module state metrics: %s", err))
			}
			delete(moduleStatusMap, moduleStatus.Name)
			continue
		}
		module := &unstructured.Unstructured{}
		module.SetGroupVersionKind(moduleStatus.Manifest.GroupVersionKind())
		module.SetName(moduleStatus.Manifest.GetName())
		module.SetNamespace(moduleStatus.Manifest.GetNamespace())
		err := moduleFunc(ctx, module)
		if apiErrors.IsNotFound(err) {
			if err := metrics.RemoveModuleStateMetrics(kyma, moduleStatus.Name); err != nil {
				logger.Info(fmt.Sprintf("error occurred while removing module state metrics: %s", err))
			}
			delete(moduleStatusMap, moduleStatus.Name)
		} else {
			moduleStatus.State = stateFromManifest(module, moduleStatus.CustomStateCheck)
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
