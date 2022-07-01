package status

import (
	"context"
	"fmt"
	"time"

	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
		k.SyncReadyConditionForModules(kyma, util.Modules{
			operatorv1alpha1.KymaKind: &util.Module{},
		}, operatorv1alpha1.ConditionStatusTrue, message)
		kyma.SetActiveChannel()
	case "":
		k.SyncReadyConditionForModules(kyma, util.Modules{
			operatorv1alpha1.KymaKind: &util.Module{},
		}, operatorv1alpha1.ConditionStatusUnknown, message)
	default:
		k.SyncReadyConditionForModules(kyma, util.Modules{
			operatorv1alpha1.KymaKind: &util.Module{},
		}, operatorv1alpha1.ConditionStatusFalse, message)
	}

	return k.Update(ctx, kyma.SetObservedGeneration())
}

func (k *Kyma) SyncReadyConditionForModules(kyma *operatorv1alpha1.Kyma, modules util.Modules, conditionStatus operatorv1alpha1.KymaConditionStatus, message string) {
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
			}
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

func (k *Kyma) GetReadyConditionForComponent(kymaObj *operatorv1alpha1.Kyma, componentName string) (*operatorv1alpha1.KymaCondition, bool) {
	status := &kymaObj.Status
	for _, existingCondition := range status.Conditions {
		if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == componentName {
			return &existingCondition, true
		}
	}

	return &operatorv1alpha1.KymaCondition{}, false
}

func (k *Kyma) UpdateConditionFromComponentState(name string, module *util.Module, kyma *operatorv1alpha1.Kyma) (bool, error) {
	updateRequired := false
	actualComponentStruct := module.Unstructured
	componentName := actualComponentStruct.GetLabels()[labels.ControllerName]

	componentStatus := actualComponentStruct.Object[watch.Status]
	if componentStatus != nil {
		condition, exists := k.GetReadyConditionForComponent(kyma, componentName)
		if !exists {
			return false, fmt.Errorf("condition not found for component %s", componentName)
		}

		switch componentStatus.(map[string]interface{})[watch.State].(string) {
		case string(operatorv1alpha1.KymaStateReady):
			if condition.Status != operatorv1alpha1.ConditionStatusTrue {
				k.SyncReadyConditionForModules(kyma, util.Modules{name: module},
					operatorv1alpha1.ConditionStatusTrue, "component ready!")

				updateRequired = true
			}

		case "":
			if condition.Status != operatorv1alpha1.ConditionStatusUnknown {
				k.SyncReadyConditionForModules(kyma, util.Modules{name: module},
					operatorv1alpha1.ConditionStatusUnknown, "component status not known!")

				updateRequired = true
			}

		default:
			if condition.Status != operatorv1alpha1.ConditionStatusFalse {
				k.SyncReadyConditionForModules(kyma, util.Modules{name: module},
					operatorv1alpha1.ConditionStatusFalse, "component not ready!")

				updateRequired = true
			}
		}
	}

	return updateRequired, nil
}
