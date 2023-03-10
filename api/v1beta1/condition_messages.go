package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Extend this list by actual needs.
const (
	// DeprecatedConditionReady was introduced by a bug. Ths condition needs to be removed on all KymaCRs.
	DeprecatedConditionReady KymaConditionType = "Ready"

	// Determines the Type of a Condition
	ConditionTypeModules       KymaConditionType = "Modules"
	ConditionTypeModuleCatalog KymaConditionType = "ModuleCatalog"
	ConditionTypeSKRWebhook    KymaConditionType = "SKRWebhook"

	// ConditionReason will be set to `Ready` on all Conditions. If the Condition is actual ready,
	// can be determined by the state.
	ConditionReason KymaConditionReason = "Ready"

	// ConditionMessage represents the current state of a Condition in a human readable format.
	ConditionMessageModuleInReadyState       = "all modules are in ready state"
	ConditionMessageModuleNotInReadyState    = "not all modules are in ready state"
	ConditionMessageModuleCatalogIsSynced    = "module catalog is synchronized"
	ConditionMessageModuleCatalogIsOutOfSync = "module catalog is out of sync and needs to be resynchronized"
	ConditionMessageSKRWebhookIsSynced       = "skrwebhook is synchronized"
	ConditionMessageSKRWebhookIsOutOfSync    = "skrwebhook is out of sync and needs to be resynchronized"
)

func GenerateMessage(conditionType KymaConditionType, status metav1.ConditionStatus) string {
	switch conditionType {
	case ConditionTypeModules:
		switch status {
		case metav1.ConditionTrue:
			return ConditionMessageModuleInReadyState
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return ConditionMessageModuleNotInReadyState
	case ConditionTypeModuleCatalog:
		switch status {
		case metav1.ConditionTrue:
			return ConditionMessageModuleCatalogIsSynced
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return ConditionMessageModuleCatalogIsOutOfSync
	case ConditionTypeSKRWebhook:
		switch status {
		case metav1.ConditionTrue:
			return ConditionMessageSKRWebhookIsSynced
		case metav1.ConditionUnknown:
		case metav1.ConditionFalse:
		}

		return ConditionMessageSKRWebhookIsOutOfSync
	}

	return "no detailed message available as condition or status is unknown to API"
}

// GetRequiredConditions returns all required Conditions for a KymaCR
func GetRequiredConditions(syncEnabled, watcherEnabled bool) []KymaConditionType {
	requiredConditions := []KymaConditionType{ConditionTypeModules}
	if syncEnabled {
		requiredConditions = append(requiredConditions, ConditionTypeModuleCatalog)
	}
	if watcherEnabled {
		requiredConditions = append(requiredConditions, ConditionTypeSKRWebhook)
	}
	return requiredConditions
}
