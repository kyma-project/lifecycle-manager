package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	OperatorPrefix    = "operator.kyma-project.io"
	ComponentPrefix   = "component.kyma-project.io"
	Separator         = "/"
	ControllerName    = OperatorPrefix + Separator + "controller-name"
	ChannelLabel      = OperatorPrefix + Separator + "channel"
	ManagedBy         = OperatorPrefix + Separator + "managed-by"
	Finalizer         = OperatorPrefix + Separator + KymaKind
	KymaName          = OperatorPrefix + Separator + "kyma-name"
	LastSync          = OperatorPrefix + Separator + "last-sync"
	Signature         = OperatorPrefix + Separator + "signature"
	ModuleName        = OperatorPrefix + Separator + "module-name"
	ProfileLabel      = OperatorPrefix + Separator + "profile"
	OverrideTypeLabel = OperatorPrefix + Separator + "override-type"
	OperatorName      = "kyma-operator"
)

func GetMatchingLabelsForModule(module *Module, profile Profile) client.MatchingLabels {
	selector := client.MatchingLabels{
		ModuleName: module.Name,
	}
	if module.ControllerName != "" {
		selector[ControllerName] = module.ControllerName
	}
	if profile != "" {
		selector[ProfileLabel] = string(profile)
	}
	return selector
}

func CheckLabelsAndFinalizers(kyma *Kyma) bool {
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
		kyma.ObjectMeta.Labels[ManagedBy] = KymaKind
		updateRequired = true
	}
	return updateRequired
}
