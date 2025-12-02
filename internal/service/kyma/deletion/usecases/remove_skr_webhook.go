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
	errFailedToDeleteSkrCertificate      = errors.New("failed to delete SKR certificate")
)

//type SKRWebhookManager interface {
//	Remove(ctx context.Context, kyma *v1beta2.Kyma) error
//}

type SkrWebhookResourcesRepository interface {
	ResourcesExist(kymaName string) (bool, error)
	DeleteWebhookResources(ctx context.Context, kymaName string) error
}

type SkrCertificateService interface {
	CertificateExists(ctx context.Context, kymaName string) (bool, error)
	DeleteSkrCertificate(ctx context.Context, kymaName string) error
}

type RemoveSkrWebhookUseCase struct {
	skrWebhookResourcesRepo SkrWebhookResourcesRepository
	skrCertificateService   SkrCertificateService
}

func NewRemoveSkrWebhookUseCase(
	skrWebhookResourcesRepo SkrWebhookResourcesRepository,
	skrCertificateService SkrCertificateService,
) *RemoveSkrWebhookUseCase {
	return &RemoveSkrWebhookUseCase{
		skrWebhookResourcesRepo: skrWebhookResourcesRepo,
		skrCertificateService:   skrCertificateService,
	}
}

func (u *RemoveSkrWebhookUseCase) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if kyma.DeletionTimestamp.IsZero() || kyma.Status.State != shared.StateDeleting {
		return false, nil
	}

	resourcesExist, err := u.skrWebhookResourcesRepo.ResourcesExist(kyma.Name)
	if err != nil {
		return false, errors.Join(err, errFailedToDetermineApplicability)
	}

	certificateExists, err := u.skrCertificateService.CertificateExists(ctx, kyma.Name)
	if err != nil {
		return false, errors.Join(err, errFailedToDetermineApplicability)
	}

	return resourcesExist || certificateExists, nil
}

func (u *RemoveSkrWebhookUseCase) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	// Delete SKR certificate first
	err := u.skrCertificateService.DeleteSkrCertificate(ctx, kyma.Name)
	if err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(err, errFailedToDeleteSkrCertificate),
		}
	}

	// Then delete webhook resources from SKR cluster
	err = u.skrWebhookResourcesRepo.DeleteWebhookResources(ctx, kyma.Name)
	if err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(err, errFailedToRemoveSkrWebhookResources),
		}
	}

	// TODO: Check when testing deletion on the cluster if skrwebhookresources.BuildSKRSecret
	// from SkrWebhookManifestManager is ok to be omitted in this usecase

	return result.Result{
		UseCase: u.Name(),
		Err:     nil,
	}
}

func (u *RemoveSkrWebhookUseCase) Name() result.UseCase {
	return usecase.DeleteSkrWatcher
}
