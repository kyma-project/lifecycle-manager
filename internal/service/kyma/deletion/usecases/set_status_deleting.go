package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

const useCase = usecase.SetKcpKymaStateDeleting
const state shared.State = shared.StateDeleting
const lastOperationMessage = "waiting for modules to be deleted"

type KymaStatusRepository interface {
	UpdateKymaStatus(ctx context.Context, kyma *v1beta2.Kyma, newState shared.State, message string) error
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
	err := u.kymaStatusRepo.UpdateKymaStatus(ctx, kyma, state, lastOperationMessage)
	return result.Result{
		UseCase: useCase,
		Err:     err,
	}
}

func (u *SetKymaStatusDeletingUseCase) Name() result.UseCase {
	return useCase
}
