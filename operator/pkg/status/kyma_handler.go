package status

import (
	"context"
	"fmt"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"time"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
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

func KymaHandler(handler Handler) *Kyma {
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
		k.AddReadyConditionForObjects(kyma, []util.ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusTrue, message)
		// set active release only when ready newState is set
		release.New(kyma, ctx).IssueChannelChangeSuccess()
		kyma.SetActiveChannel()
	case "":
		k.AddReadyConditionForObjects(kyma, []util.ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusUnknown, message)
	default:
		k.AddReadyConditionForObjects(kyma, []util.ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusFalse, message)
	}
	return k.Update(ctx, kyma.SetObservedGeneration())

}

func (k *Kyma) AddReadyConditionForObjects(kymaObj *operatorv1alpha1.Kyma, typesByTemplate []util.ComponentsAssociatedWithTemplate, conditionStatus operatorv1alpha1.KymaConditionStatus, message string) {
	status := &kymaObj.Status
	for _, typeByTemplate := range typesByTemplate {
		condition, exists := k.GetReadyConditionForComponent(kymaObj, typeByTemplate.ComponentName)
		if !exists {
			condition = &operatorv1alpha1.KymaCondition{
				Type:   operatorv1alpha1.ConditionTypeReady,
				Reason: typeByTemplate.ComponentName,
			}
			status.Conditions = append(status.Conditions, *condition)
		}
		condition.TemplateInfo = operatorv1alpha1.TemplateInfo{
			Channel:    typeByTemplate.TemplateChannel,
			Generation: typeByTemplate.TemplateGeneration,
		}
		condition.LastTransitionTime = &metav1.Time{Time: time.Now()}
		condition.Message = message
		condition.Status = conditionStatus

		for i, existingCondition := range status.Conditions {
			if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == typeByTemplate.ComponentName {
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

func (k *Kyma) UpdateReadyCondition(kymaObj *operatorv1alpha1.Kyma, componentNames []string, conditionStatus operatorv1alpha1.KymaConditionStatus, message string) {
	status := kymaObj.Status
	for _, componentName := range componentNames {
		condition, exists := k.GetReadyConditionForComponent(kymaObj, componentName)
		if !exists {
			continue
		}
		condition.LastTransitionTime = &metav1.Time{Time: time.Now()}
		condition.Message = message
		condition.Status = conditionStatus

		for i, existingCondition := range status.Conditions {
			if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == componentName {
				status.Conditions[i] = *condition
				break
			}
		}
	}
}

func (k *Kyma) UpdateComponentConditions(actualComponentStruct *unstructured.Unstructured, kyma *operatorv1alpha1.Kyma) (bool, error) {
	updateRequired := false
	componentStatus := actualComponentStruct.Object[watch.Status]
	componentName := actualComponentStruct.GetLabels()[labels.ControllerName]
	if componentStatus != nil {
		condition, exists := k.GetReadyConditionForComponent(kyma, componentName)
		if !exists {
			return false, fmt.Errorf("condition not found for component %s", componentName)
		}

		switch componentStatus.(map[string]interface{})[watch.State].(string) {

		case string(operatorv1alpha1.KymaStateReady):
			if condition.Status != operatorv1alpha1.ConditionStatusTrue {
				k.UpdateReadyCondition(kyma, []string{componentName},
					operatorv1alpha1.ConditionStatusTrue, "component ready!")
				// "istio", "serverless" are hardcoded, remove!
				k.UpdateReadyCondition(kyma, []string{"istio", "serverless"},
					operatorv1alpha1.ConditionStatusTrue, "component ready!")
				updateRequired = true
			}

		case "":
			if condition.Status != operatorv1alpha1.ConditionStatusUnknown {
				k.UpdateReadyCondition(kyma, []string{componentName},
					operatorv1alpha1.ConditionStatusUnknown, "component status not known!")
				updateRequired = true
			}

		default:
			if condition.Status != operatorv1alpha1.ConditionStatusFalse {
				k.UpdateReadyCondition(kyma, []string{componentName},
					operatorv1alpha1.ConditionStatusFalse, "component not ready!")
				updateRequired = true
			}
		}
	}
	return updateRequired, nil
}
