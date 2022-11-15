package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	OperatorPrefix = "operator.kyma-project.io"
	Separator      = "/"
	ControllerName = OperatorPrefix + Separator + "controller-name"
	ChannelLabel   = OperatorPrefix + Separator + "channel"
	ManagedBy      = OperatorPrefix + Separator + "managed-by"
	Finalizer      = OperatorPrefix + Separator + string(KymaKind)
	KymaName       = OperatorPrefix + Separator + "kyma-name"
	Signature      = OperatorPrefix + Separator + "signature"
	ModuleName     = OperatorPrefix + Separator + "module-name"
	OperatorName   = "lifecycle-manager"
	OwnedByLabel   = OperatorPrefix + Separator + "owned-by"
	OwnedByFormat  = "%s__%s"
	WatchedByLabel = OperatorPrefix + Separator + "watched-by"
)

func GetMatchingLabelsForModule(module *Module) client.MatchingLabels {
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
