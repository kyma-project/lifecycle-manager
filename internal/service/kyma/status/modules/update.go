package modules

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type (
	GetModuleFunc     func(ctx context.Context, module client.Object) error
	RemoveMetricsFunc func(kymaName, moduleName string)
)

type ModuleStatusGenerator interface {
	GenerateModuleStatus(module *modulecommon.Module, currentStatus *v1beta2.ModuleStatus) (v1beta2.ModuleStatus, error)
}

var errNilKyma = fmt.Errorf("kyma object is nil")

type StatusService struct {
	statusGenerator   ModuleStatusGenerator
	kcpClient         client.Client
	removeMetricsFunc RemoveMetricsFunc
}

func NewModulesStatusService(statusGenerator ModuleStatusGenerator, client client.Client, removeMetricsFunc RemoveMetricsFunc) *StatusService {
	return &StatusService{
		statusGenerator:   statusGenerator,
		kcpClient:         client,
		removeMetricsFunc: removeMetricsFunc,
	}
}

func (m *StatusService) UpdateModuleStatuses(ctx context.Context, kyma *v1beta2.Kyma, modules modulecommon.Modules) error {
	if kyma == nil {
		return errNilKyma
	}

	moduleStatusMap := kyma.GetModuleStatusMap()
	for _, module := range modules {
		moduleStatus, exists := moduleStatusMap[module.ModuleName]
		newModuleStatus, err := m.statusGenerator.GenerateModuleStatus(module, moduleStatus)
		if err != nil {
			return fmt.Errorf("failed to generate module status for module %s: %w", module.ModuleName, err)
		}
		if exists {
			*moduleStatus = newModuleStatus
		} else {
			kyma.Status.Modules = append(kyma.Status.Modules, newModuleStatus)
		}
	}

	DeleteNoLongerExistingModuleStatus(ctx, kyma, m.getModule, m.removeMetricsFunc)
	//moduleStatusMap = kyma.GetModuleStatusMap()
	//moduleStatusesToBeDeletedFromKymaStatus := kyma.GetNoLongerExistingModuleStatus()
	//for _, moduleStatus := range moduleStatusesToBeDeletedFromKymaStatus {
	//	if moduleStatus.Manifest == nil {
	//		m.removeMetricsFunc(kyma.Name, moduleStatus.Name)
	//		delete(moduleStatusMap, moduleStatus.Name)
	//		continue
	//	}
	//	manifestCR := moduleStatus.GetManifestCR()
	//	err := m.getModule(ctx, manifestCR)
	//	if util.IsNotFound(err) {
	//		m.removeMetricsFunc(kyma.Name, moduleStatus.Name)
	//		delete(moduleStatusMap, moduleStatus.Name)
	//	} else {
	//		moduleStatus.State = stateFromManifest(manifestCR)
	//	}
	//}
	//kyma.Status.Modules = convertToNewModuleStatus(moduleStatusMap)

	return nil
}

func DeleteNoLongerExistingModuleStatus(ctx context.Context, kyma *v1beta2.Kyma, getModule GetModuleFunc, removeMetrics RemoveMetricsFunc) {
	moduleStatusMap := kyma.GetModuleStatusMap()
	moduleStatusesToBeDeletedFromKymaStatus := kyma.GetNoLongerExistingModuleStatus()
	for _, moduleStatus := range moduleStatusesToBeDeletedFromKymaStatus {
		if moduleStatus.Manifest == nil {
			removeMetrics(kyma.Name, moduleStatus.Name)
			delete(moduleStatusMap, moduleStatus.Name)
			continue
		}
		manifestCR := moduleStatus.GetManifestCR()
		err := getModule(ctx, manifestCR)
		if util.IsNotFound(err) {
			removeMetrics(kyma.Name, moduleStatus.Name)
			delete(moduleStatusMap, moduleStatus.Name)
		} else {
			moduleStatus.State = stateFromManifest(manifestCR)
		}
	}
	kyma.Status.Modules = convertToNewModuleStatus(moduleStatusMap)
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
