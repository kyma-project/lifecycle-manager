package status

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/kyma-operator/operator/pkg/parsed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
)

var ErrTemplateNotFound = errors.New("template not found")

type Kyma struct {
	client.StatusWriter
}

type Handler interface {
	Status() client.StatusWriter
}

func Helper(handler Handler) *Kyma {
	return &Kyma{StatusWriter: handler.Status()}
}

func (k *Kyma) UpdateStatusForExistingModules(ctx context.Context,
	kyma *operatorv1alpha1.Kyma, newState operatorv1alpha1.State,
) error {
	kyma.Status.State = newState

	switch newState {
	case operatorv1alpha1.StateReady:
		kyma.SetActiveChannel()
	case "":
	case operatorv1alpha1.StateDeleting:
	case operatorv1alpha1.StateError:
	case operatorv1alpha1.StateProcessing:
	default:
	}

	if err := k.Update(ctx, kyma.SetObservedGeneration()); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	return nil
}

func (k *Kyma) SyncReadyConditionForModules(kyma *operatorv1alpha1.Kyma, modules parsed.Modules,
	conditionStatus operatorv1alpha1.KymaConditionStatus, message string,
) {
	status := &kyma.Status

	for name := range modules {
		condition, exists := k.GetReadyConditionForComponent(kyma, name)
		if !exists {
			status.Conditions = append(status.Conditions, *condition)
		}

		condition.LastTransitionTime = &metav1.Time{Time: time.Now()}
		condition.Message = message
		condition.Status = conditionStatus
	}
}

func (k *Kyma) SyncModuleInfo(kyma *operatorv1alpha1.Kyma, modules parsed.Modules) {
	moduleInfoMap := kyma.GetModuleInfoMap()

	for _, module := range modules {
		latestModuleInfo := operatorv1alpha1.ModuleInfo{
			ModuleName: module.Name,
			Name:       module.Unstructured.GetName(),
			Namespace:  module.Unstructured.GetNamespace(),
			TemplateInfo: operatorv1alpha1.TemplateInfo{
				Channel:    module.Template.Spec.Channel,
				Generation: module.Template.Generation,
				GroupVersionKind: metav1.GroupVersionKind{
					Group:   module.GroupVersionKind().Group,
					Version: module.GroupVersionKind().Version,
					Kind:    module.GroupVersionKind().Kind,
				},
			},
		}
		moduleInfo, exists := moduleInfoMap[module.Name]
		if exists {
			*moduleInfo = latestModuleInfo
		} else {
			kyma.Status.ModuleInfos = append(kyma.Status.ModuleInfos, latestModuleInfo)
		}
	}
}

func (k *Kyma) GetTemplateInfoForModule(
	kyma *operatorv1alpha1.Kyma,
	moduleName string,
) (*operatorv1alpha1.TemplateInfo, error) {
	for i := range kyma.Status.ModuleInfos {
		moduleInfo := &kyma.Status.ModuleInfos[i]
		if moduleInfo.ModuleName == moduleName {
			return &moduleInfo.TemplateInfo, nil
		}
	}
	// should not happen
	return nil, ErrTemplateNotFound
}

func (k *Kyma) GetReadyConditionForComponent(kymaObj *operatorv1alpha1.Kyma,
	componentName string,
) (*operatorv1alpha1.KymaCondition, bool) {
	status := &kymaObj.Status
	for i := range status.Conditions {
		existingCondition := &status.Conditions[i]
		if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == componentName {
			return existingCondition, true
		}
	}

	return &operatorv1alpha1.KymaCondition{
		Type:   operatorv1alpha1.ConditionTypeReady,
		Reason: componentName,
	}, false
}

func (k *Kyma) UpdateConditionFromComponentState(name string, module *parsed.Module,
	kyma *operatorv1alpha1.Kyma,
) (bool, error) {
	updateRequired := false
	component := module.Unstructured
	moduleName := component.GetLabels()[operatorv1alpha1.ModuleName]

	status := component.Object[watch.Status]
	if status != nil {
		condition, exists := k.GetReadyConditionForComponent(kyma, moduleName)
		if !exists {
			// In rare cases, the condition for a module does not yet exist in an update
			// this can happen if the creation failed due to a resource version conflict
			// if this happens we need to make sure to create it dynamically
			k.SyncReadyConditionForModules(kyma, parsed.Modules{name: module},
				operatorv1alpha1.ConditionStatusFalse, "component tracked!")
			updateRequired = true
			return updateRequired, nil
		}

		switch status.(map[string]interface{})[watch.State].(string) {
		case string(operatorv1alpha1.StateReady):
			if condition.Status != operatorv1alpha1.ConditionStatusTrue {
				k.SyncReadyConditionForModules(kyma, parsed.Modules{name: module},
					operatorv1alpha1.ConditionStatusTrue, "component ready!")

				updateRequired = true
			}

		case "":
			if condition.Status != operatorv1alpha1.ConditionStatusUnknown {
				k.SyncReadyConditionForModules(kyma, parsed.Modules{name: module},
					operatorv1alpha1.ConditionStatusUnknown, "component status not known!")

				updateRequired = true
			}

		default:
			// This will only trigger an update on the conditions in case the component was not set to not false before
			if condition.Status != operatorv1alpha1.ConditionStatusFalse {
				k.SyncReadyConditionForModules(kyma, parsed.Modules{name: module},
					operatorv1alpha1.ConditionStatusFalse, "component not ready!")

				updateRequired = true
			}
		}
	}

	return updateRequired, nil
}
