package shared

import "strings"

const (
	KymaGroup      = "kyma-project.io"
	OperatorGroup  = "operator." + KymaGroup
	Separator      = "/"
	ControllerName = OperatorGroup + Separator + "controller-name"
	ChannelLabel   = OperatorGroup + Separator + "channel"
	// ManagedBy defines the controller managing the resource.
	//
	// Deprecated: The 'operator.kyma-project.io/managed-by' label is deprecated.
	// Use the spec.manager field in the Watcher CR instead.
	// This label is maintained for backward compatibility and will be removed in a future release.
	ManagedBy           = OperatorGroup + Separator + "managed-by"
	ManagedByLabelValue = kymaValue
	kymaValue           = "kyma"

	IstioInjectionLabel = "istio-injection"
	WardenLabel         = "namespaces.warden.kyma-project.io/validate"
	EnabledValue        = "enabled"
	KymaName            = OperatorGroup + Separator + "kyma-name"
	ModuleName          = OperatorGroup + Separator + "module-name"
	IsMandatoryModule   = OperatorGroup + Separator + "mandatory-module"

	OperatorName = "lifecycle-manager"
	// WatchedByLabel defines a redirect to a controller that should be getting a notification
	// if this resource is changed.
	WatchedByLabel      = OperatorGroup + Separator + "watched-by"
	WatchedByLabelValue = kymaValue
	// PurposeLabel defines the purpose of the resource, i.e. Secrets which will be used to certificate management.
	PurposeLabel = OperatorGroup + Separator + "purpose"
	CertManager  = "klm-watcher-cert-manager"
	// SkipReconcileLabel indicates this specific resource will be skipped during reconciliation.
	SkipReconcileLabel    = OperatorGroup + Separator + "skip-reconciliation"
	UnmanagedKyma         = "unmanaged-kyma"
	DefaultRemoteKymaName = "default"

	InternalLabel = OperatorGroup + Separator + "internal"
	BetaLabel     = OperatorGroup + Separator + "beta"

	GlobalAccountIDLabel = KymaGroup + Separator + "global-account-id"
	SubAccountIDLabel    = KymaGroup + Separator + "subaccount-id"
	RegionLabel          = KymaGroup + Separator + "region"
	PlatformRegionLabel  = KymaGroup + Separator + "platform-region"
	PlanLabel            = KymaGroup + Separator + "broker-plan-name"
	RuntimeIDLabel       = KymaGroup + Separator + "runtime-id"

	EnableLabelValue  = "true"
	DisableLabelValue = "false"
)

func IsEnabled(value string) bool {
	return strings.ToLower(value) == EnableLabelValue
}
