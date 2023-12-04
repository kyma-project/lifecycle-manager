package v1beta2

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func EnsureLabelsAndFinalizers(kyma *v1beta2.Kyma) bool {
	if controllerutil.ContainsFinalizer(kyma, "foregroundDeletion") {
		return false
	}

	updateRequired := false
	if kyma.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(kyma, shared.Finalizer) {
		controllerutil.AddFinalizer(kyma, shared.Finalizer)
		updateRequired = true
	}

	if kyma.ObjectMeta.Labels == nil {
		kyma.ObjectMeta.Labels = make(map[string]string)
	}

	if _, ok := kyma.ObjectMeta.Labels[shared.ManagedBy]; !ok {
		kyma.ObjectMeta.Labels[shared.ManagedBy] = shared.OperatorName
		updateRequired = true
	}
	return updateRequired
}
