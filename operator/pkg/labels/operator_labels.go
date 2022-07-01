package labels

import (
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	OperatorPrefix  = "operator.kyma-project.io"
	ComponentPrefix = "component.kyma-project.io"
	Separator       = "/"
	ControllerName  = OperatorPrefix + Separator + "controller-name"
	Channel         = OperatorPrefix + Separator + "channel"
	ManagedBy       = OperatorPrefix + Separator + "managed-by"
	Finalizer       = OperatorPrefix + Separator + v1alpha1.KymaKind
	KymaName        = OperatorPrefix + Separator + "kyma-name"
	LastSync        = OperatorPrefix + Separator + "last-sync"
)

func CheckLabelsAndFinalizers(kyma *v1alpha1.Kyma) bool {
	updateRequired := false
	if !controllerutil.ContainsFinalizer(kyma, Finalizer) {
		controllerutil.AddFinalizer(kyma, Finalizer)
		updateRequired = true
	}

	if kyma.ObjectMeta.Labels == nil {
		kyma.ObjectMeta.Labels = make(map[string]string, 0)
	}

	if _, ok := kyma.ObjectMeta.Labels[ManagedBy]; !ok {
		kyma.ObjectMeta.Labels[ManagedBy] = v1alpha1.KymaKind
		updateRequired = true
	}
	return updateRequired
}
