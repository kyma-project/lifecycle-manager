package certmanager

import (
	"context"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type CertRepository interface {
	Renew(ctx context.Context, name string) error
}

type Service struct {
	certRepo CertRepository
}

func NewService(certRepo CertRepository) *Service {
	return &Service{
		certRepo: certRepo,
	}
}

func (s *Service) Renew(ctx context.Context, name string) error {
	if err := s.certRepo.Renew(ctx, name); err != nil {
		return fmt.Errorf("failed to renew SKR certificate secret: %w", err)
	}

	return nil
}

// SkrSecretNeedsRenewal checks if the SKR Secret needs to be renewed.
// Renewal is required if the gateway certificate secret is newer than the SKR certificate secret.
func (s *Service) SkrSecretNeedsRenewal(gatewaySecret *apicorev1.Secret, clientCertNotBefore time.Time) bool {
	gwSecretLastModifiedAtValue, ok := gatewaySecret.Annotations[shared.LastModifiedAtAnnotation]
	// always renew if the annotation is not set
	if !ok {
		return true
	}

	gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue)
	// always renew if unable to parse
	if err != nil {
		return true
	}

	return clientCertNotBefore.Before(gwSecretLastModifiedAt)
}
