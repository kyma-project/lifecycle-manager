package kyma

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) EnsureLabelsAndFinalizers(kyma *v1beta2.Kyma) bool {
	if controllerutil.ContainsFinalizer(kyma, "foregroundDeletion") {
		return false
	}

	updateRequired := false
	if kyma.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(kyma, shared.KymaFinalizer) {
		controllerutil.AddFinalizer(kyma, shared.KymaFinalizer)
		updateRequired = true
	}

	if kyma.Labels == nil {
		kyma.Labels = make(map[string]string)
	}

	if _, ok := kyma.Labels[shared.ManagedBy]; !ok {
		kyma.Labels[shared.ManagedBy] = shared.OperatorName
		updateRequired = true
	}
	return updateRequired
}
