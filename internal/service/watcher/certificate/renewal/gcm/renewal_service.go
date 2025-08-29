package gcm

import (
	"context"
	"fmt"
	"time"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type CertificateRepository interface {
	Get(ctx context.Context, name string) (*gcertv1alpha1.Certificate, error)
	Update(ctx context.Context, cert *gcertv1alpha1.Certificate) error
}

type Service struct {
	certRepo CertificateRepository
}

func NewService(certRepo CertificateRepository) *Service {
	return &Service{
		certRepo: certRepo,
	}
}

func (s *Service) Renew(ctx context.Context, name string) error {
	var cert *gcertv1alpha1.Certificate
	var err error
	if cert, err = s.certRepo.Get(ctx, name); err != nil || cert == nil {
		return fmt.Errorf("could not get certificate for renewal: %w", err)
	}

	if cert.Spec.EnsureRenewedAfter != nil {
		cert.Spec.EnsureRenewedAfter = nil
	}

	cert.Spec.Renew = boolPtr(true)

	if err = s.certRepo.Update(ctx, cert); err != nil {
		return fmt.Errorf("failed to update certificate for renewal: %w", err)
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

	lastRequestedAtValue, ok := skrSecret.Annotations[shared.GCMSecretAnnotation]
        // always renew if the annotation is not set
	if !ok {
		return true
	}

	lastRequestedAtValueTime, err := time.Parse(time.RFC3339, lastRequestedAtValue)
        // always renew if unable to parse
	if err != nil {
		return true
	}

	return lastRequestedAtValueTime.Before(gwSecretLastModifiedAt)
}

func boolPtr(b bool) *bool {
	return &b
}
