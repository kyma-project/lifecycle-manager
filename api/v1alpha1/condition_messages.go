package v1alpha1

import (
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

func GenerateMessage(reason KymaConditionReason, status metav1.ConditionStatus) string {
	switch reason {
	case ConditionReasonModulesAreReady:
		switch status {
		case metav1.ConditionTrue:
			return MessageModuleInReadyState
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return MessageModuleNotInReadyState
	case ConditionReasonModuleCatalogIsReady:
		switch status {
		case metav1.ConditionTrue:
			return MessageModuleCatalogIsSynced
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return MessageModuleCatalogIsOutOfSync
	case ConditionReasonSKRWebhookIsReady:
		switch status {
		case metav1.ConditionTrue:
			return MessageSKRWebhookIsSynced
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return MessageSKRWebhookIsOutOfSync
	}

	return "no detailed message available as reason is unknown to API"
}
