package certmanager

import (
	"context"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type SecretRepository interface {
	Delete(ctx context.Context, name string) error
}

type Service struct {
	secretRepository SecretRepository
}

func NewService(certRepo SecretRepository) *Service {
	return &Service{
		secretRepository: certRepo,
	}
}

func (s *Service) Renew(ctx context.Context, name string) error {
	if err := s.secretRepository.Delete(ctx, name); err != nil {
		return fmt.Errorf("failed to renew SKR certificate secret. Deletion failed: %w", err)
	}

	return nil
}

// SkrSecretNeedsRenewal checks if the SKR Secret needs to be renewed.
// Renewal is required if the gateway certificate secret is newer than the SKR certificate secret.
func (s *Service) SkrSecretNeedsRenewal(gatewaySecret, skrSecret *apicorev1.Secret) bool {
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

	return skrSecret.CreationTimestamp.Time.Before(gwSecretLastModifiedAt)
}
