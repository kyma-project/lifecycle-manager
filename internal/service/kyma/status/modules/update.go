package modules

import (
	"context"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type GetModuleFunc func(ctx context.Context, module client.Object) error
type RemoveMetricsFunc func(kymaName, moduleName string)

type StatusService struct {
	kcpClient         client.Client
	removeMetricsFunc RemoveMetricsFunc
}

func NewModulesStatusService(client client.Client, removeMetricsFunc RemoveMetricsFunc) *StatusService {
	return &StatusService{
		kcpClient:         client,
		removeMetricsFunc: removeMetricsFunc,
	}
}

func (m *StatusService) UpdateStatusModule(ctx context.Context, kyma *v1beta2.Kyma, modules common.Modules) {
	updateModuleStatusFromExistingModules(kyma, modules)
	m.deleteNoLongerExistingModuleStatus(ctx, kyma)
}

func updateModuleStatusFromExistingModules(kyma *v1beta2.Kyma, modules common.Modules) {
	moduleStatusMap := kyma.GetModuleStatusMap()
	for _, module := range modules {
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
	if module.Template.Err != nil {
		return generateModuleStatusFromError(module, existStatus)
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
		Channel: module.Template.DesiredChannel,
		Version: manifestObject.Spec.Version,
		Manifest: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       manifestObject.GetName(),
				Namespace:  manifestObject.GetNamespace(),
				Generation: manifestObject.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: manifestKind, APIVersion: manifestAPIVersion},
		},
		Template: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       module.Template.GetName(),
				Namespace:  module.Template.GetNamespace(),
				Generation: module.Template.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: templateKind, APIVersion: templateAPIVersion},
		},
		Resource: moduleResource,
	}

	if module.IsUnmanaged {
		moduleStatus.State = shared.StateUnmanaged
		moduleStatus.Manifest = nil
		moduleStatus.Template = nil
		moduleStatus.Resource = nil
	}

	return moduleStatus
}

func generateModuleStatusFromError(module *common.Module, existStatus *v1beta2.ModuleStatus) v1beta2.ModuleStatus {
	switch {
	case errors.Is(module.Template.Err, templatelookup.ErrTemplateUpdateNotAllowed):
		newModuleStatus := existStatus.DeepCopy()
		newModuleStatus.State = shared.StateWarning
		newModuleStatus.Message = module.Template.Err.Error()
		return *newModuleStatus
	case errors.Is(module.Template.Err, moduletemplateinfolookup.ErrNoTemplatesInListResult):
		return v1beta2.ModuleStatus{
			Name:    module.ModuleName,
			Channel: module.Template.DesiredChannel,
			FQDN:    module.FQDN,
			State:   shared.StateWarning,
			Message: module.Template.Err.Error(),
		}
	case errors.Is(module.Template.Err, moduletemplateinfolookup.ErrWaitingForNextMaintenanceWindow):
		newModuleStatus := existStatus.DeepCopy()
		newModuleStatus.Message = module.Template.Err.Error()
		return *newModuleStatus
	case errors.Is(module.Template.Err, moduletemplateinfolookup.ErrFailedToDetermineIfMaintenanceWindowIsActive):
		newModuleStatus := existStatus.DeepCopy()
		newModuleStatus.Message = module.Template.Err.Error()
		newModuleStatus.State = shared.StateError
		return *newModuleStatus
	default:
		return v1beta2.ModuleStatus{
			Name:    module.ModuleName,
			Channel: module.Template.DesiredChannel,
			FQDN:    module.FQDN,
			State:   shared.StateError,
			Message: module.Template.Err.Error(),
		}
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
func (m *StatusService) deleteNoLongerExistingModuleStatus(ctx context.Context, kyma *v1beta2.Kyma) {
	moduleStatusMap := kyma.GetModuleStatusMap()
	moduleStatusesToBeDeletedFromKymaStatus := kyma.GetNoLongerExistingModuleStatus()
	for idx := range moduleStatusesToBeDeletedFromKymaStatus {
		moduleStatus := moduleStatusesToBeDeletedFromKymaStatus[idx]
		if moduleStatus.Manifest == nil {
			m.removeMetricsFunc(kyma.Name, moduleStatus.Name)
			delete(moduleStatusMap, moduleStatus.Name)
			continue
		}
		manifestCR := moduleStatus.GetManifestCR()
		err := m.getModule(ctx, manifestCR)
		if util.IsNotFound(err) {
			m.removeMetricsFunc(kyma.Name, moduleStatus.Name)
			delete(moduleStatusMap, moduleStatus.Name)
		} else {
			moduleStatus.State = stateFromManifest(manifestCR)
		}
	}
	kyma.Status.Modules = convertToNewModuleStatus(moduleStatusMap)
}

func (m *StatusService) getModule(ctx context.Context, module client.Object) error {
	err := m.kcpClient.Get(ctx, client.ObjectKey{Namespace: module.GetNamespace(), Name: module.GetName()}, module)
	if err != nil {
		return fmt.Errorf("failed to get module by name-namespace: %w", err)
	}
	return nil
}

func convertToNewModuleStatus(moduleStatusMap map[string]*v1beta2.ModuleStatus) []v1beta2.ModuleStatus {
	newModuleStatus := make([]v1beta2.ModuleStatus, 0)
	for _, moduleStatus := range moduleStatusMap {
		newModuleStatus = append(newModuleStatus, *moduleStatus)
	}
	return newModuleStatus
}
