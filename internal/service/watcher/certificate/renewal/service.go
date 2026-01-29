package renewal

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

var ErrGatewaySecretMissingLastModifiedAt = errors.New("gateway secret is missing lastModifiedAt annotation")

type CertificateRepository interface {
	Renew(ctx context.Context, name string) error
	GetValidity(ctx context.Context, name string) (time.Time, time.Time, error)
}

type SecretRepository interface {
	Get(ctx context.Context, name string) (*apicorev1.Secret, error)
}

type Service struct {
	certRepo          CertificateRepository
	secretRepo        SecretRepository
	gatewaySecretName string
}

func NewService(certRepo CertificateRepository, secretRepo SecretRepository, gatewaySecretName string) *Service {
	return &Service{
		certRepo:          certRepo,
		secretRepo:        secretRepo,
		gatewaySecretName: gatewaySecretName,
	}
}

// RenewSkrCertificate renews the SKR client certificate.
func (s *Service) RenewSkrCertificate(ctx context.Context, certName string) error {
	if err := s.certRepo.Renew(ctx, certName); err != nil {
		return fmt.Errorf("failed to renew SKR certificate: %w", err)
	}

	return nil
}

// SkrCertificateNeedsRenewal checks if the SKR client certificate needs to be renewed.
// Renewal is required if the gateway certificate secret is newer than the SKR client certificate.
func (s *Service) SkrCertificateNeedsRenewal(ctx context.Context, certName string) (bool, error) {
	certNotBefore, _, err := s.certRepo.GetValidity(ctx, certName)
	if err != nil {
		return false, fmt.Errorf("failed to determine SKR client certificate validity: %w", err)
	}

	gatewaySecretLastModifiedAt, err := s.getGatewaySecretLastModifiedAt(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to determine gateway secret lastModifiedAt: %w", err)
	}

	return certNotBefore.Before(gatewaySecretLastModifiedAt), nil
}

func (s *Service) getGatewaySecretLastModifiedAt(ctx context.Context) (time.Time, error) {
	gatewaySecret, err := s.secretRepo.Get(ctx, s.gatewaySecretName)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get gateway secret: %w", err)
	}

	lastModifiedAtValue, ok := gatewaySecret.Annotations[shared.LastModifiedAtAnnotation]
	if !ok {
		return time.Time{}, ErrGatewaySecretMissingLastModifiedAt
	}

	lastModifiedAt, err := time.Parse(time.RFC3339, lastModifiedAtValue)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse gateway secret lastModifiedAt annotation: %w", err)
	}

	return lastModifiedAt, nil
}
