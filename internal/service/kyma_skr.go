package service

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

type KymaSKRService struct {
	kcpClient         client.Client
	skrContextService *SKRContextService
}

func NewKymaSKRService(kcpClient client.Client, skrContextService *SKRContextService) *KymaSKRService {
	return &KymaSKRService{kcpClient: kcpClient, skrContextService: skrContextService}
}

func (s KymaSKRService) DropKyma(ctx context.Context, kyma types.NamespacedName) error {
	skrContext, err := s.skrContextService.GetCache(kyma)
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}

	s.skrContextService.InvalidateCache(kyma)
	if err = skrContext.RemoveFinalizersFromKyma(ctx); client.IgnoreNotFound(err) != nil {
		return err
	}
	return nil
}

func (r *KymaSKRService) SyncStatusToRemote(ctx context.Context, kcpKyma *v1beta2.Kyma) error {
	remoteKyma, err := r.fetchRemoteKyma(ctx, kcpKyma)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			// remote kyma not found because it's deleted, should not continue
			return nil
		}
		return err
	}

	skrContext, err := r.skrContextService.GetCache(kcpKyma.GetNamespacedName())
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}

	if err := skrContext.SynchronizeKymaMetadata(ctx, kcpKyma, remoteKyma); err != nil {
		return fmt.Errorf("failed to sync SKR Kyma CR Metadata: %w", err)
	}

	if err := skrContext.SynchronizeKymaStatus(ctx, kcpKyma, remoteKyma); err != nil {
		return fmt.Errorf("failed to sync SKR Kyma CR Status: %w", err)
	}

	return nil
}

func (r *KymaSKRService) fetchRemoteKyma(ctx context.Context,
	kcpKyma *v1beta2.Kyma) (*v1beta2.Kyma, error) {
	syncContext, err := r.skrContextService.GetCache(kcpKyma.GetNamespacedName())
	if err != nil {
		return nil, fmt.Errorf("failed to get syncContext: %w", err)
	}
	remoteKyma, err := syncContext.CreateOrFetchKyma(ctx, r.kcpClient, kcpKyma)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			return nil, err
		}
		return nil, fmt.Errorf("could not create or fetch remote kyma: %w", err)
	}
	return remoteKyma, nil
}
