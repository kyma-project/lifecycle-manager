package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type KymaRepo interface {
	DropAllFinalizers(ctx context.Context, kymaName string) error
}

type DropKymaFinalizers struct {
	kymaRepo KymaRepo
}

func NewDropKymaFinalizers(kymaRepo KymaRepo) *DropKymaFinalizers {
	return &DropKymaFinalizers{
		kymaRepo: kymaRepo,
	}
}

func (u *DropKymaFinalizers) IsApplicable(ctx context.Context, kcpKyma *v1beta2.Kyma) (bool, error) {
	return len(kcpKyma.GetFinalizers()) > 0, nil
}

func (u *DropKymaFinalizers) Execute(ctx context.Context, kcpKyma *v1beta2.Kyma) result.Result {
	return result.Result{
		UseCase: u.Name(),
		Err:     u.kymaRepo.DropAllFinalizers(ctx, kcpKyma.GetName()),
	}
}

func (u *DropKymaFinalizers) Name() result.UseCase {
	return usecase.DropKymaFinalizers
}
