package v1beta2

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Extend this list by actual needs.
const (
	// DeprecatedConditionTypeReady was introduced by a bug. Ths condition needs to be removed on all KymaCRs.
	DeprecatedConditionTypeReady KymaConditionType = "Ready"
	ConditionTypeModules         KymaConditionType = "Modules"
	ConditionTypeModuleCatalog   KymaConditionType = "ModuleCatalog"
	ConditionTypeSKRWebhook      KymaConditionType = "SKRWebhook"

	// ConditionReason will be set to `Ready` on all Conditions. If the Condition is actual ready,
	// can be determined by the state.
	ConditionReason KymaConditionReason = "Ready"

	ConditionMessageModuleInReadyState        = "all modules are in ready state"
	ConditionMessageModuleNotInReadyState     = "not all modules are in ready state"
	ConditionMessageModuleCatalogIsSynced     = "module templates are synchronized"
	ConditionMessageModuleCatalogIsOutOfSync  = "module templates are out of sync and need to be resynchronized"
	ConditionMessageSKRWebhookIsSynced        = "skrwebhook is synchronized"
	ConditionMessageSKRWebhookIsOutOfSync     = "skrwebhook is out of sync and needs to be resynchronized"
	ConditionMessageModuleStateUnknown        = "modules state is unknown"
	ConditionMessageModuleCatalogStateUnknown = "module templates synchronization state is unknown"
)

func GenerateMessage(conditionType KymaConditionType, status apimetav1.ConditionStatus) string {
	switch conditionType {
	case ConditionTypeModules:
		switch status {
		case apimetav1.ConditionTrue:
			return ConditionMessageModuleInReadyState
		case apimetav1.ConditionUnknown:
			return ConditionMessageModuleStateUnknown
		case apimetav1.ConditionFalse:
		}

		return ConditionMessageModuleNotInReadyState
	case ConditionTypeModuleCatalog:
		switch status {
		case apimetav1.ConditionTrue:
			return ConditionMessageModuleCatalogIsSynced
		case apimetav1.ConditionUnknown:
			return ConditionMessageModuleCatalogStateUnknown
		case apimetav1.ConditionFalse:
		}

		return ConditionMessageModuleCatalogIsOutOfSync
	case ConditionTypeSKRWebhook:
		switch status {
		case apimetav1.ConditionTrue:
			return ConditionMessageSKRWebhookIsSynced
		case apimetav1.ConditionUnknown:
		case apimetav1.ConditionFalse:
		}

		return ConditionMessageSKRWebhookIsOutOfSync
	case DeprecatedConditionTypeReady:
	}

	return "no detailed message available as condition or status is unknown to API"
}

// GetRequiredConditionTypes returns all required ConditionTypes for a KymaCR.
func GetRequiredConditionTypes(syncEnabled, watcherEnabled bool) []KymaConditionType {
	requiredConditions := []KymaConditionType{ConditionTypeModules}
	if syncEnabled {
		requiredConditions = append(requiredConditions, ConditionTypeModuleCatalog)
	}
	if watcherEnabled {
		requiredConditions = append(requiredConditions, ConditionTypeSKRWebhook)
	}
	return requiredConditions
}
