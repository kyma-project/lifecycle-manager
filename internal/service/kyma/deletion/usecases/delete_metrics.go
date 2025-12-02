package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type KymaMetrics interface {
	HasMetrics(kymaName string) (bool, error)
	CleanupMetrics(kymaName string)
}

type DeleteMetrics struct {
	kymaMetrics KymaMetrics
}

func NewDeleteMetrics(kymaMetrics KymaMetrics) *DeleteMetrics {
	return &DeleteMetrics{
		kymaMetrics: kymaMetrics,
	}
}

func (u *DeleteMetrics) IsApplicable(_ context.Context, kyma *v1beta2.Kyma) (bool, error) {
	hasMetrics, err := u.kymaMetrics.HasMetrics(kyma.GetName())
	if err != nil {
		return false, err
	}

	return hasMetrics, nil
}

func (u *DeleteMetrics) Execute(_ context.Context, kyma *v1beta2.Kyma) result.Result {
	u.kymaMetrics.CleanupMetrics(kyma.GetName())
	return result.Result{UseCase: u.Name()}
}

func (u *DeleteMetrics) Name() result.UseCase {
	return usecase.DeleteMetrics
}
