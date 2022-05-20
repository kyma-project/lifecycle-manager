package status

import (
	"context"
	"time"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"github.com/kyma-project/kyma-operator/operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Kyma struct {
	client.StatusWriter
	record.EventRecorder
}

func (h *Kyma) UpdateStatus(
	ctx context.Context,
	kyma *operatorv1alpha1.Kyma,
	newState operatorv1alpha1.KymaState,
	message string,
) error {
	kyma.Status.State = newState
	switch newState {
	case operatorv1alpha1.KymaStateReady:
		h.AddReadyConditionForObjects(kyma, []util.ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusTrue, message)
		// set active release only when ready newState is set
		release.New(kyma.Status.ActiveChannel, kyma.Spec.Channel, h.GetEventAdapter(kyma)).IssueChannelChangeSuccess()
		kyma.SetActiveChannel()
	case "":
		h.AddReadyConditionForObjects(kyma, []util.ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusUnknown, message)
	default:
		h.AddReadyConditionForObjects(kyma, []util.ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusFalse, message)
	}
	return h.Update(ctx, kyma.SetObservedGeneration())

}

func (h *Kyma) AddReadyConditionForObjects(kymaObj *operatorv1alpha1.Kyma, typesByTemplate []util.ComponentsAssociatedWithTemplate, conditionStatus operatorv1alpha1.KymaConditionStatus, message string) {
	status := &kymaObj.Status
	for _, typeByTemplate := range typesByTemplate {
		condition, exists := h.GetReadyConditionForComponent(kymaObj, typeByTemplate.ComponentName)
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

func (h *Kyma) GetReadyConditionForComponent(kymaObj *operatorv1alpha1.Kyma, componentName string) (*operatorv1alpha1.KymaCondition, bool) {
	status := &kymaObj.Status
	for _, existingCondition := range status.Conditions {
		if existingCondition.Type == operatorv1alpha1.ConditionTypeReady && existingCondition.Reason == componentName {
			return &existingCondition, true
		}
	}
	return &operatorv1alpha1.KymaCondition{}, false
}

func (h *Kyma) GetEventAdapter(kyma *operatorv1alpha1.Kyma) adapter.Eventing {
	return func(eventtype, reason, message string) {
		h.Event(kyma, eventtype, reason, message)
	}
}

func (h *Kyma) UpdateReadyCondition(kymaObj *operatorv1alpha1.Kyma, componentNames []string, conditionStatus operatorv1alpha1.KymaConditionStatus, message string) {
	status := kymaObj.Status
	for _, componentName := range componentNames {
		condition, exists := h.GetReadyConditionForComponent(kymaObj, componentName)
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
