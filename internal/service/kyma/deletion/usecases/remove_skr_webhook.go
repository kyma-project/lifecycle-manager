package usecases

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

type SKRWebhookManager interface {
	Remove(ctx context.Context, kyma *v1beta2.Kyma) error
}

type SkrWebhookResourceRepo interface {
	ResourcesExist(kymaName string) (bool, error)
	DeleteWebhookResources(ctx context.Context, kymaName string) error
}

type RemoveSkrWebhookUseCase struct {
	skrWebhookManager SKRWebhookManager
}

func NewRemoveSkrWebhookUseCase(skrWebhookManager SKRWebhookManager) *RemoveSkrWebhookUseCase {
	return &RemoveSkrWebhookUseCase{
		skrWebhookManager: skrWebhookManager,
	}
}

func (u *RemoveSkrWebhookUseCase) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if kyma.DeletionTimestamp.IsZero() || kyma.Status.State != shared.StateDeleting {
		return false, nil
	}

	if {

	}

}

func (u *RemoveSkrWebhookUseCase) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	if u.skrWebhookManager == nil {
		return result.Result{
			UseCase: u.Name(),
		}
	}

	err := u.skrWebhookManager.Remove(ctx, kyma)
	if err != nil {

	}
	return result.Result{
		UseCase: u.Name(),
		Err:     err,
	}
}

func (u *RemoveSkrWebhookUseCase) Name() result.UseCase {
	return usecase.DeleteSkrWatcher
}
