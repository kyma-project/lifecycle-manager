package shared

const (
	OperatorGroup  = "operator.kyma-project.io"
	Separator      = "/"
	ControllerName = OperatorGroup + Separator + "controller-name"
	ChannelLabel   = OperatorGroup + Separator + "channel"
	// ManagedBy defines the controller managing the resource.
	ManagedBy = OperatorGroup + Separator + "managed-by"

	KymaName               = OperatorGroup + Separator + "kyma-name"
	Signature              = OperatorGroup + Separator + "signature"
	ModuleName             = OperatorGroup + Separator + "module-name"
	IsRemoteModuleTemplate = OperatorGroup + Separator + "remote-template"

	//nolint:gosec // OCI registry credits label, no confidential content
	OCIRegistryCredLabel = "oci-registry-cred"
	OperatorName         = "lifecycle-manager"
	// WatchedByLabel defines a redirect to a controller that should be getting a notification
	// if this resource is changed.
	WatchedByLabel = OperatorGroup + Separator + "watched-by"
	// PurposeLabel defines the purpose of the resource, i.e. Secrets which will be used to certificate management.
	PurposeLabel = OperatorGroup + Separator + "purpose"
	CertManager  = "klm-watcher-cert-manager"
	// SkipReconcileLabel indicates this specific resource will be skipped during reconciliation.
	SkipReconcileLabel    = OperatorGroup + Separator + "skip-reconciliation"
	UnmanagedKyma         = "unmanaged-kyma"
	DefaultRemoteKymaName = "default"
	InternalLabel         = OperatorGroup + Separator + "internal"
	BetaLabel             = OperatorGroup + Separator + "beta"

	// Controls ModuleTemplate sync logic.
	// If put on the Kyma object, allows to disable sync for all ModuleTemplatesByLabel
	// If put on a single ModuleTemplate, allows to disable sync just for this object.
	SyncLabel = OperatorGroup + Separator + "sync"
)
