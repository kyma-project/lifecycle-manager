package v1beta2

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:gosec // no secrets defined in these consts
const (
	// DeprecatedConditionTypeReady was introduced by a bug. Ths condition needs to be removed on all KymaCRs.
	DeprecatedConditionTypeReady KymaConditionType = "Ready"
	ConditionTypeModules         KymaConditionType = "Modules"
	ConditionTypeModuleCatalog   KymaConditionType = "ModuleCatalog"
	ConditionTypeCRDsSync        KymaConditionType = "CRDsSync"
	ConditionTypeSKRWebhook      KymaConditionType = "SKRWebhook"

	ConditionTypeSKRImagePullSecretSync KymaConditionType = "SKRImagePullSecretSync"

	// ConditionReason will be set to `Ready` on all Conditions. If the Condition is actual ready,
	// can be determined by the state.
	ConditionReason KymaConditionReason = "Ready"

	ConditionMessageModuleInReadyState          = "all modules are in ready state"
	ConditionMessageModuleNotInReadyState       = "not all modules are in ready state"
	ConditionMessageModuleCatalogIsSynced       = "module templates are synchronized"
	ConditionMessageModuleCatalogIsOutOfSync    = "module templates are out of sync and need to be resynchronized"
	ConditionMessageModuleStateUnknown          = "modules state is unknown"
	ConditionMessageModuleCatalogStateUnknown   = "module templates synchronization state is unknown"
	ConditionMessageCRDsAreSynced               = "CRDs are synchronized to the SKR"
	ConditionMessageCRDsAreNotSynced            = "CRDs are not synchronized to the SKR"
	ConditionMessageCRDsSyncStateUnknown        = "CRDs synchronization state is unknown"
	ConditionMessageSKRWebhookIsSynced          = "skrwebhook is synchronized"
	ConditionMessageSKRWebhookIsOutOfSync       = "skrwebhook is out of sync and needs to be resynchronized"
	ConditionMessageSKRImagePullSecretSynced    = "skr image pull secret is synchronized"
	ConditionMessageSKRImagePullSecretOutOfSync = "skr image pull secret is out of sync and needs to be resynchronized"
)

func GenerateMessage(conditionType KymaConditionType, status apimetav1.ConditionStatus) string {
	messages, ok := conditionMessagesByType[conditionType]
	if !ok {
		return defaultConditionMessage
	}

	switch status {
	case apimetav1.ConditionTrue:
		return messages.trueMessage
	case apimetav1.ConditionUnknown:
		if messages.unknownMessage != "" {
			return messages.unknownMessage
		}
	case apimetav1.ConditionFalse:
	}

	return messages.falseMessage
}

const defaultConditionMessage = "no detailed message available as condition or status is unknown to API"

type conditionMessages struct {
	trueMessage    string
	falseMessage   string
	unknownMessage string
}

//nolint:gochecknoglobals // lookup table for condition messages
var conditionMessagesByType = map[KymaConditionType]conditionMessages{
	ConditionTypeModules: {
		trueMessage:    ConditionMessageModuleInReadyState,
		falseMessage:   ConditionMessageModuleNotInReadyState,
		unknownMessage: ConditionMessageModuleStateUnknown,
	},
	ConditionTypeModuleCatalog: {
		trueMessage:    ConditionMessageModuleCatalogIsSynced,
		falseMessage:   ConditionMessageModuleCatalogIsOutOfSync,
		unknownMessage: ConditionMessageModuleCatalogStateUnknown,
	},
	ConditionTypeCRDsSync: {
		trueMessage:    ConditionMessageCRDsAreSynced,
		falseMessage:   ConditionMessageCRDsAreNotSynced,
		unknownMessage: ConditionMessageCRDsSyncStateUnknown,
	},
	ConditionTypeSKRWebhook: {
		trueMessage:  ConditionMessageSKRWebhookIsSynced,
		falseMessage: ConditionMessageSKRWebhookIsOutOfSync,
	},
	ConditionTypeSKRImagePullSecretSync: {
		trueMessage:  ConditionMessageSKRImagePullSecretSynced,
		falseMessage: ConditionMessageSKRImagePullSecretOutOfSync,
	},
}

// GetRequiredConditionTypes returns all required ConditionTypes for a KymaCR.
func GetRequiredConditionTypes(watcherEnabled, skrImagePullSecretSyncEnabled bool) []KymaConditionType {
	requiredConditions := []KymaConditionType{ConditionTypeModules}
	requiredConditions = append(requiredConditions, ConditionTypeModuleCatalog)
	requiredConditions = append(requiredConditions, ConditionTypeCRDsSync)

	if watcherEnabled {
		requiredConditions = append(requiredConditions, ConditionTypeSKRWebhook)
	}
	if skrImagePullSecretSyncEnabled {
		requiredConditions = append(requiredConditions, ConditionTypeSKRImagePullSecretSync)
	}
	return requiredConditions
}
