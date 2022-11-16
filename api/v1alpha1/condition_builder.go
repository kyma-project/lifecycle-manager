package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionBuilder struct {
	Status             metav1.ConditionStatus
	Reason             KymaConditionReason
	ObservedGeneration int64
}

const (
	MessageModuleInReadyState       = "all modules are in ready state"
	MessageModuleNotInReadyState    = "not all modules are in ready state"
	MessageModuleCatalogIsSynced    = "module catalog is synchronized"
	MessageModuleCatalogIsOutOfSync = "module catalog is out of sync and needs to be resynchronized"
	MessageSKRWebhookIsSynced       = "skrwebhook is synchronized"
	MessageSKRWebhookIsOutOfSync    = "skrwebhook is out of sync and needs to be resynchronized"
)

// Extend this list by actual needs.
const (
	ConditionReasonModulesAreReady      KymaConditionReason = "ModulesAreReady"
	ConditionReasonModuleCatalogIsReady KymaConditionReason = "ModuleCatalogIsReady"
	ConditionReasonSKRWebhookIsReady    KymaConditionReason = "SKRWebhookIsReady"
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

func (cb *ConditionBuilder) SetObservedGeneration(observedGeneration int64) *ConditionBuilder {
	cb.ObservedGeneration = observedGeneration
	return cb
}

func (cb *ConditionBuilder) Build() metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionTypeReady),
		Status:             cb.Status,
		LastTransitionTime: metav1.Time{Time: time.Now()},
		Reason:             string(cb.Reason),
		Message:            cb.generateMessage(),
		ObservedGeneration: cb.ObservedGeneration,
	}
}

func (cb *ConditionBuilder) generateMessage() string {
	switch cb.Reason {
	case ConditionReasonModulesAreReady:
		switch cb.Status {
		case metav1.ConditionTrue:
			return MessageModuleInReadyState
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return MessageModuleNotInReadyState
	case ConditionReasonModuleCatalogIsReady:
		switch cb.Status {
		case metav1.ConditionTrue:
			return MessageModuleCatalogIsSynced
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return MessageModuleCatalogIsOutOfSync
	case ConditionReasonSKRWebhookIsReady:
		switch cb.Status {
		case metav1.ConditionTrue:
			return MessageSKRWebhookIsSynced
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return MessageSKRWebhookIsOutOfSync
	}

	return "no detailed message available as reason is unknown to API"
}
