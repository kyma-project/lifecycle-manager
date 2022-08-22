package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionBuilder struct {
	Status metav1.ConditionStatus
	Reason KymaConditionReason
}

const (
	MessageModuleInReadyState        = "all modules are in ready state"
	MessageModuleNotInReadyState     = "not all modules are in ready state"
	MessageModuleInUnidentifiedState = "modules are in unidentified state"
)

func NewConditionBuilder() *ConditionBuilder {
	return &ConditionBuilder{}
}

func (cb *ConditionBuilder) SetStatus(status metav1.ConditionStatus) *ConditionBuilder {
	cb.Status = status
	return cb
}

func (cb *ConditionBuilder) SetReason(reason KymaConditionReason) *ConditionBuilder {
	cb.Reason = reason
	return cb
}

func (cb *ConditionBuilder) Build() metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionTypeReady),
		Status:             cb.Status,
		LastTransitionTime: metav1.Time{Time: time.Now()},
		Reason:             string(cb.Reason),
		Message:            cb.generateMessage(),
	}
}

func (cb *ConditionBuilder) generateMessage() string {
	if cb.Reason == ConditionReasonModulesAreReady {
		if cb.Status == metav1.ConditionTrue {
			return MessageModuleInReadyState
		}
		if cb.Status == metav1.ConditionFalse || cb.Status == metav1.ConditionUnknown {
			return MessageModuleNotInReadyState
		}
	}
	return MessageModuleInUnidentifiedState
}
