package modules

import (
	"context"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules/generator/fromerror"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type GetModuleFunc func(ctx context.Context, module client.Object) error
type RemoveMetricsFunc func(kymaName, moduleName string)

type ModuleStatusGenerator interface {
	GenerateModuleStatus(module *common.Module, currentStatus *v1beta2.ModuleStatus) v1beta2.ModuleStatus
}

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

func (m *StatusService) UpdateModuleStatuses(ctx context.Context, kyma *v1beta2.Kyma, modules common.Modules) {
	if kyma == nil {
		return
	}

	moduleStatusMap := kyma.GetModuleStatusMap()
	for _, module := range modules {
		moduleStatus, exists := moduleStatusMap[module.ModuleName]

		newModuleStatus := generateModuleStatus(module, moduleStatus)
		if exists {
			*moduleStatus = newModuleStatus
		} else {
			kyma.Status.Modules = append(kyma.Status.Modules, newModuleStatus)
		}
	}

	moduleStatusMap = kyma.GetModuleStatusMap()
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

func generateModuleStatus(module *common.Module, currentStatus *v1beta2.ModuleStatus) v1beta2.ModuleStatus {
	if module.Template.Err != nil {
		return fromerror.GenerateModuleStatusFromError(module.Template.Err, module.ModuleName, module.Template.DesiredChannel, module.FQDN, currentStatus)
	}

	manifestObject := module.Manifest
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

	manifestAPIVersion, manifestKind := manifestObject.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	templateAPIVersion, templateKind := module.Template.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
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
