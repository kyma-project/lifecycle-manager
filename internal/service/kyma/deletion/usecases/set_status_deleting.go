package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

const useCase = usecase.SetKcpKymaStateDeleting

type KymaStatusRepository interface {
	UpdateStatusDeleting(ctx context.Context, kyma *v1beta2.Kyma) error
}

type SetKymaStatusDeletingUseCase struct {
	kymaStatusRepo KymaStatusRepository
}

func NewSetKymaStatusDeletingUseCase(kymaStatusRepo KymaStatusRepository) *SetKymaStatusDeletingUseCase {
	return &SetKymaStatusDeletingUseCase{
		kymaStatusRepo: kymaStatusRepo,
	}
}

func (u *SetKymaStatusDeletingUseCase) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	return !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != shared.StateDeleting, nil
}

func (u *SetKymaStatusDeletingUseCase) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	err := u.kymaStatusRepo.UpdateStatusDeleting(ctx, kyma)
	return result.Result{
		UseCase: useCase,
		Err:     err,
	}
}

func (u *SetKymaStatusDeletingUseCase) Name() result.UseCase {
	return useCase
}
