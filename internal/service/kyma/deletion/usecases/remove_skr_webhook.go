package usecases

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
)

var (
	//nolint:revive // no better formatting for this
	errFailedToDetermineApplicability    = errors.New("failed to determine applicability for removing SKR webhook resources")
	errFailedToRemoveSkrWebhookResources = errors.New("failed to remove SKR webhook resources")
)

type SkrWebhookResourcesRepository interface {
	ResourcesExist(ctx context.Context, kymaName types.NamespacedName) (bool, error)
	DeleteWebhookResources(ctx context.Context, kymaName types.NamespacedName) error
}

type RemoveSkrWebhookResources struct {
	skrWebhookResourcesRepo SkrWebhookResourcesRepository
}

func NewRemoveSkrWebhookResources(
	skrWebhookResourcesRepo SkrWebhookResourcesRepository,
) *RemoveSkrWebhookResources {
	return &RemoveSkrWebhookResources{
		skrWebhookResourcesRepo: skrWebhookResourcesRepo,
	}
}

func (u *RemoveSkrWebhookResources) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if kyma.DeletionTimestamp.IsZero() || kyma.Status.State != shared.StateDeleting {
		return false, nil
	}

	resourcesExist, err := u.skrWebhookResourcesRepo.ResourcesExist(ctx, kyma.GetNamespacedName())
	if err != nil {
		return false, errors.Join(errFailedToDetermineApplicability, err)
	}

	return resourcesExist, nil
}

func (u *RemoveSkrWebhookResources) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	// Delete webhook resources from SKR cluster
	err := u.skrWebhookResourcesRepo.DeleteWebhookResources(ctx, kyma.GetNamespacedName())
	if err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(errFailedToRemoveSkrWebhookResources, err),
		}
	}

	return result.Result{
		UseCase: u.Name(),
		Err:     nil,
	}
}

func (u *RemoveSkrWebhookResources) Name() result.UseCase {
	return usecase.DeleteSkrWebhookResources
}
