package certmanager

import (
	"context"
	"fmt"
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
