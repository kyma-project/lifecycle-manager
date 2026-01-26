package usecases

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/name"
)

var (
	//nolint:revive // no better formatting for this
	errFailedToDetermineWatcherCleanupApplicability = errors.New("failed to determine applicability for removing SKR certificate")
	errFailedToDeleteWatcherSkrCertificate          = errors.New("failed to delete SKR certificate")
	errFailedToDeleteWatcherSkrSecret               = errors.New("failed to delete SKR secret")
)

type SecretRepository interface {
	Exists(ctx context.Context, name string) (bool, error)
	Delete(ctx context.Context, name string) error
}

type CertificateRepository interface {
	Exists(ctx context.Context, certName string) (bool, error)
	Delete(ctx context.Context, name string) error
}

type DeleteWatcherCertificateSetup struct {
	certRepo   CertificateRepository
	secretRepo SecretRepository
}

func NewDeleteWatcherCertificateSetup(
	certRepo CertificateRepository,
	secretRepo SecretRepository,
) *DeleteWatcherCertificateSetup {
	return &DeleteWatcherCertificateSetup{
		certRepo:   certRepo,
		secretRepo: secretRepo,
	}
}

func (u *DeleteWatcherCertificateSetup) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if kyma.DeletionTimestamp.IsZero() || kyma.Status.State != shared.StateDeleting {
		return false, nil
	}
	certName := name.SkrCertificate(kyma.Name)
	certificateExists, err := u.certRepo.Exists(ctx, certName)
	if err != nil {
		return false, errors.Join(errFailedToDetermineWatcherCleanupApplicability, err)
	}
	if certificateExists {
		return true, nil
	}

	secretExists, err := u.secretRepo.Exists(ctx, certName)
	if err != nil {
		return false, errors.Join(errFailedToDetermineWatcherCleanupApplicability, err)
	}

	return secretExists, nil
}

func (u *DeleteWatcherCertificateSetup) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	certName := name.SkrCertificate(kyma.Name)

	if err := u.certRepo.Delete(ctx, certName); err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(errFailedToDeleteWatcherSkrCertificate, err),
		}
	}
	if err := u.secretRepo.Delete(ctx, certName); err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(errFailedToDeleteWatcherSkrSecret, err),
		}
	}

	return result.Result{
		UseCase: u.Name(),
		Err:     nil,
	}
}

func (u *DeleteWatcherCertificateSetup) Name() result.UseCase {
	return usecase.DeleteWatcherCertificateSetup
}
