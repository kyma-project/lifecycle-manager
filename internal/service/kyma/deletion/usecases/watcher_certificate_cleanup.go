package usecases

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/name"
)

var (
	errFailedToDetermineWatcherCleanupApplicability = errors.New("failed to determine applicability for removing SKR certificate")
	errFailedToDeleteWatcherSkrCertificate          = errors.New("failed to delete SKR certificate")
	errFailedToDeleteWatcherSkrSecret               = errors.New("failed to delete SKR secret")
)

type SecretRepository interface {
	Exists(ctx context.Context, name string) (bool, error)
	Delete(ctx context.Context, name string) error
}

type WatcherCertificateCleanup struct {
	certRepo   certificate.CertificateRepository
	secretRepo SecretRepository
}

func NewWatcherCertificateCleanup(
	certRepo certificate.CertificateRepository,
	secretRepo SecretRepository,
) *WatcherCertificateCleanup {
	return &WatcherCertificateCleanup{
		certRepo:   certRepo,
		secretRepo: secretRepo,
	}
}

func (u *WatcherCertificateCleanup) IsApplicable(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if kyma.DeletionTimestamp.IsZero() || kyma.Status.State != shared.StateDeleting {
		return false, nil
	}
	certName := name.SkrCertificate(kyma.Name)
	certificateExists, err := u.certRepo.Exists(ctx, certName)
	if err != nil {
		return false, errors.Join(err, errFailedToDetermineWatcherCleanupApplicability)
	}
	if certificateExists {
		return true, nil
	}

	secretExists, err := u.secretRepo.Exists(ctx, certName)
	if err != nil {
		return false, errors.Join(err, errFailedToDetermineWatcherCleanupApplicability)
	}

	return secretExists, nil
}

func (u *WatcherCertificateCleanup) Execute(ctx context.Context, kyma *v1beta2.Kyma) result.Result {
	// Delete SKR certificate from KCP cluster
	if err := u.certRepo.Delete(ctx, kyma.Name); err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(err, errFailedToDeleteWatcherSkrCertificate),
		}
	}
	if err := u.secretRepo.Delete(ctx, kyma.Name); err != nil {
		return result.Result{
			UseCase: u.Name(),
			Err:     errors.Join(err, errFailedToDeleteWatcherSkrSecret),
		}
	}

	return result.Result{
		UseCase: u.Name(),
		Err:     nil,
	}
}

func (u *WatcherCertificateCleanup) Name() result.UseCase {
	return usecase.DeleteWatcherCertificateSetup
}
