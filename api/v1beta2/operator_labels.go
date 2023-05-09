package v1beta2

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	OperatorPrefix = "operator.kyma-project.io"
	Separator      = "/"
	ControllerName = OperatorPrefix + Separator + "controller-name"
	ChannelLabel   = OperatorPrefix + Separator + "channel"
	// ManagedBy defines the controller managing the resource.
	ManagedBy      = OperatorPrefix + Separator + "managed-by"
	Finalizer      = OperatorPrefix + Separator + string(KymaKind)
	PurgeFinalizer = OperatorPrefix + Separator + "purge-finalizer"
	KymaName       = OperatorPrefix + Separator + "kyma-name"
	Signature      = OperatorPrefix + Separator + "signature"
	ModuleName     = OperatorPrefix + Separator + "module-name"
	// Notice: This label is intended solely for testing purposes and should not be used in production module templates.
	UseLocalTemplate = OperatorPrefix + Separator + "use-local-template"

	//nolint:gosec
	OCIRegistryCredLabel = "oci-registry-cred"
	OperatorName         = "lifecycle-manager"
	// WatchedByLabel defines a redirect to a controller that should be getting a notification
	// if this resource is changed.
	WatchedByLabel = OperatorPrefix + Separator + "watched-by"
	// PurposeLabel defines the purpose of the resource, i.e. Secrets which will be used to certificate management.
	PurposeLabel = OperatorPrefix + Separator + "purpose"
	CertManager  = "klm-watcher-cert-manager"
	// SkipReconcileLabel indicates this specific resource will be skipped during reconciliation.
	SkipReconcileLabel = OperatorPrefix + Separator + "skip-reconciliation"
	UnmanagedKyma      = "unmanaged-kyma"

	InternalLabel = OperatorPrefix + Separator + "internal"
	BetaLabel     = OperatorPrefix + Separator + "beta"

	// Controls ModuleTemplate sync logic.
	// If put on the Kyma object, allows to disable sync for all ModuleTemplatesByLabel
	// If put on a single ModuleTemplate, allows to disable sync just for this object.
	SyncLabel = OperatorPrefix + Separator + "sync"
)

func ModuleTemplatesByLabel(module *Module) client.MatchingLabels {
	selector := client.MatchingLabels{
		ModuleName: module.Name,
	}
	if module.ControllerName != "" {
		selector[ControllerName] = module.ControllerName
	}
	return selector
}

func (kyma *Kyma) CheckLabelsAndFinalizers() bool {
	if controllerutil.ContainsFinalizer(kyma, "foregroundDeletion") {
		return false
	}

	updateRequired := false
	if kyma.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(kyma, Finalizer) {
		controllerutil.AddFinalizer(kyma, Finalizer)
		updateRequired = true
	}

	if kyma.ObjectMeta.Labels == nil {
		kyma.ObjectMeta.Labels = make(map[string]string)
	}

	if _, ok := kyma.ObjectMeta.Labels[ManagedBy]; !ok {
		kyma.ObjectMeta.Labels[ManagedBy] = OperatorName
		updateRequired = true
	}
	return updateRequired
}
