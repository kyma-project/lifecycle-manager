package gcm

import (
	"context"
	"fmt"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
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

func boolPtr(b bool) *bool {
	return &b
}
