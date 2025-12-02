package usecases

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

var (
	errFailedToDetermineApplicability    = errors.New("failed to determine applicability for removing SKR webhook resources")
	errFailedToRemoveSkrWebhookResources = errors.New("failed to remove SKR webhook resources")
)

//type SKRWebhookManager interface {
//	Remove(ctx context.Context, kyma *v1beta2.Kyma) error
//}

type SkrWebhookResourcesRepository interface {
	ResourcesExist(kymaName string) (bool, error)
	DeleteWebhookResources(ctx context.Context, kymaName string) error
}

type RemoveSkrWebhookUseCase struct {
	skrWebhookResourcesRepo SkrWebhookResourcesRepository
}

func NewRemoveSkrWebhookUseCase(skrWebhookResourcesRepo SkrWebhookResourcesRepository) *RemoveSkrWebhookUseCase {
	return &RemoveSkrWebhookUseCase{
		skrWebhookResourcesRepo: skrWebhookResourcesRepo,
	}
}

func (u *RemoveSkrWebhookUseCase) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if kyma.DeletionTimestamp.IsZero() || kyma.Status.State != shared.StateDeleting {
		return false, nil
	}

	ok, err := u.skrWebhookResourcesRepo.ResourcesExist(kyma.Name)
	if err != nil {
		return false, errors.Join(err, errFailedToDetermineApplicability)
	}

	return !ok, nil
}

func (u *RemoveSkrWebhookUseCase) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	err := u.skrWebhookResourcesRepo.DeleteWebhookResources(ctx, kyma.Name)
	if err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(err, errFailedToRemoveSkrWebhookResources),
		}
	}

	return result.Result{
		UseCase: u.Name(),
		Err:     nil,
	}
}

func (u *RemoveSkrWebhookUseCase) Name() result.UseCase {
	return usecase.DeleteSkrWatcher
}
