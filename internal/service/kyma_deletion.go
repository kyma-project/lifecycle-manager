package service

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

var ErrManifestsStillExist = errors.New("manifests still exist")

type KymaDeletionService struct {
	kymaRepository     *repository.KymaRepository
	manifestRepository *repository.ManifestRepository
	skrWebhookManager  *SKRWebhookManagerService
	kymaService        *KymaService
	skrContextService  *SKRContextService
	Metrics            *metrics.KymaMetrics
}

func NewKymaDeletionService(client client.Client, manager *SKRWebhookManagerService,
	KymaService *KymaService, Metrics *metrics.KymaMetrics) *KymaDeletionService {
	return &KymaDeletionService{
		kymaRepository:     repository.NewKymaRepository(client),
		manifestRepository: repository.NewManifestRepository(client),
		skrWebhookManager:  manager,
		kymaService:        KymaService,
		Metrics:            Metrics,
	}
}

func (r *KymaDeletionService) HandleOrphanedResourcesDeletion(ctx context.Context, kymaName string) error {
	if err := r.skrWebhookManager.RemoveKCPCertificate(ctx, kymaName); err != nil {
		return err
	}

	return nil
}

func (r *KymaDeletionService) DeleteRemoteKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	if r.kymaService.SyncKymaEnabled(kyma) {
		skrContext, err := r.skrContextService.GetCache(kyma.GetNamespacedName())
		if err != nil {
			return fmt.Errorf("failed to get skrContext: %w", err)
		}
		if err := skrContext.DeleteKyma(ctx); client.IgnoreNotFound(err) != nil {
			logf.FromContext(ctx).V(log.InfoLevel).Error(err, "Failed to be deleted remotely!")
			return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
		}
		logf.FromContext(ctx).V(log.InfoLevel).Info("Successfully deleted remotely!")
	}
	return nil
}

func (r *KymaDeletionService) CleanupKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	r.Metrics.CleanupMetrics(kyma.Name)
	r.RemoveAllFinalizers(kyma)

	if err := r.kymaService.UpdateKyma(ctx, kyma); err != nil {
		return fmt.Errorf("error while updating kyma during deletion: %w", err)
	}
	return nil
}

func (r *KymaDeletionService) CleanupManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) error {
	relatedManifests, err := r.kymaService.GetRelatedManifestCRs(ctx, kyma)
	if err != nil {
		return fmt.Errorf("error while trying to get manifests: %w", err)
	}

	if r.relatedManifestCRsAreDeleted(relatedManifests) {
		return nil
	}

	if err = r.manifestRepository.RemoveManifests(ctx, relatedManifests); err != nil {
		return fmt.Errorf("error while trying to delete manifests: %w", err)
	}
	return ErrManifestsStillExist
}

func (r *KymaDeletionService) relatedManifestCRsAreDeleted(manifests []v1beta2.Manifest) bool {
	return len(manifests) == 0
}

func (r *KymaDeletionService) RemoveAllFinalizers(kyma *v1beta2.Kyma) {
	for _, finalizer := range kyma.Finalizers {
		controllerutil.RemoveFinalizer(kyma, finalizer)
	}
}
