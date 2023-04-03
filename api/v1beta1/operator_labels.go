package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

const (
	OperatorPrefix = "operator.kyma-project.io"
	Separator      = "/"
	ControllerName = OperatorPrefix + Separator + "controller-name"
	ChannelLabel   = OperatorPrefix + Separator + "channel"
	// ManagedBy defines the controller managing the resource.
	ManagedBy  = OperatorPrefix + Separator + "managed-by"
	Finalizer  = OperatorPrefix + Separator + string(KymaKind)
	KymaName   = OperatorPrefix + Separator + "kyma-name"
	Signature  = OperatorPrefix + Separator + "signature"
	ModuleName = OperatorPrefix + Separator + "module-name"
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

	// ChannelWhitelistPrefix is used to identify labels for whitelisting a channel
	ChannelWhitelistPrefix  = OperatorPrefix + Separator + "whitelist-channel-"
	ChannelWhitelistEnabled = "true"
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
	if !controllerutil.ContainsFinalizer(kyma, Finalizer) {
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

func (kyma *Kyma) GetWhitelistedChannels() []string {
	var whitelistChannels []string

	for label, value := range kyma.ObjectMeta.Labels {
		if channelName, prefixFound := strings.CutPrefix(label, ChannelWhitelistPrefix); prefixFound {
			if value == ChannelWhitelistEnabled {
				whitelistChannels = append(whitelistChannels, channelName)
			}
		}
	}

	return whitelistChannels
}
