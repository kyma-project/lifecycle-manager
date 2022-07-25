package status

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/kyma-operator/operator/pkg/parsed"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrConditionNotFound = errors.New("condition not found")

type Kyma struct {
	client.StatusWriter
}

type Handler interface {
	Status() client.StatusWriter
}

func Helper(handler Handler) *Kyma {
	return &Kyma{StatusWriter: handler.Status()}
}

func (k *Kyma) UpdateStatus(
	ctx context.Context,
	kyma *operatorv1alpha1.Kyma,
	newState operatorv1alpha1.KymaState,
	message string,
) error {
	kyma.Status.State = newState

	switch newState {
	case operatorv1alpha1.KymaStateReady:
		k.SyncReadyConditionForModules(kyma, parsed.Modules{
			operatorv1alpha1.KymaKind: &parsed.Module{},
		}, operatorv1alpha1.ConditionStatusTrue, message)
		kyma.SetActiveChannel()
	case "":
		k.SyncReadyConditionForModules(kyma, parsed.Modules{
			operatorv1alpha1.KymaKind: &parsed.Module{},
		}, operatorv1alpha1.ConditionStatusUnknown, message)
	case operatorv1alpha1.KymaStateDeleting:
	case operatorv1alpha1.KymaStateError:
	case operatorv1alpha1.KymaStateProcessing:
	default:
		k.SyncReadyConditionForModules(kyma, parsed.Modules{
			operatorv1alpha1.KymaKind: &parsed.Module{},
		}, operatorv1alpha1.ConditionStatusFalse, message)
	}

	if err := k.Update(ctx, kyma.SetObservedGeneration()); err != nil {
		return fmt.Errorf("conditions could not be updated: %w", err)
	}

	return nil
}

func (k *Kyma) SyncReadyConditionForModules(kyma *operatorv1alpha1.Kyma, modules parsed.Modules,
	conditionStatus operatorv1alpha1.KymaConditionStatus, message string,
) {
	status := &kyma.Status

	for name, module := range modules {
		condition, exists := k.GetReadyConditionForComponent(kyma, name)
		if !exists {
			condition = &operatorv1alpha1.KymaCondition{
				Type:   operatorv1alpha1.ConditionTypeReady,
				Reason: name,
			}
			status.Conditions = append(status.Conditions, *condition)
		}

		if module.Template != nil {
			condition.TemplateInfo = operatorv1alpha1.TemplateInfo{
				Channel:    module.Template.Spec.Channel,
				Generation: module.Template.Generation,
				GroupVersionKind: metav1.GroupVersionKind{
					Group:   module.GroupVersionKind().Group,
					Version: module.GroupVersionKind().Version,
					Kind:    module.GroupVersionKind().Kind,
				},
			}
		}

		// Here should make sure module.Unstructured come from actual module CR but not from module template
		if module.Unstructured != nil {
			condition.ModuleInfo = operatorv1alpha1.ModuleInfo{Generation: module.Unstructured.GetGeneration()}
		}

		condition.LastTransitionTime = &metav1.Time{Time: time.Now()}
		condition.Message = message
		condition.Status = conditionStatus

		for i, existingCondition := range status.Conditions {
			if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == name {
				status.Conditions[i] = *condition

				break
			}
		}
	}
}

func (k *Kyma) GetReadyConditionForComponent(kymaObj *operatorv1alpha1.Kyma,
	componentName string,
) (*operatorv1alpha1.KymaCondition, bool) {
	status := &kymaObj.Status
	for _, existingCondition := range status.Conditions {
		if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == componentName {
			return &existingCondition, true
		}
	}

	return &operatorv1alpha1.KymaCondition{}, false
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
			return false, fmt.Errorf("error with condition for component %s: %w",
				moduleName, ErrConditionNotFound)
		}

		switch status.(map[string]interface{})[watch.State].(string) {
		case string(operatorv1alpha1.KymaStateReady):
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
			if condition.Status != operatorv1alpha1.ConditionStatusFalse {
				k.SyncReadyConditionForModules(kyma, parsed.Modules{name: module},
					operatorv1alpha1.ConditionStatusFalse, "component not ready!")

				updateRequired = true
			}
		}
	}

	return updateRequired, nil
}
