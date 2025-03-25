package service

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
)

const metricsError event.Reason = "MetricsError"

type KymaService struct {
	kymaRepository     *repository.KymaRepository
	manifestRepository *repository.ManifestRepository
	inKCPMode          bool
	isManagedKyma      bool
	metrics            *metrics.KymaMetrics
	event.Event
}

func NewKymaService(client client.Client, inKCPMode, isManagedKyma bool) *KymaService {
	return &KymaService{
		kymaRepository: repository.NewKymaRepository(client),
		inKCPMode:      inKCPMode,
		isManagedKyma:  isManagedKyma,
	}
}

func (s *KymaService) InitConditions(kyma *v1beta2.Kyma, watcherEnabled bool) {
	status.InitConditions(kyma, s.SyncKymaEnabled(kyma), watcherEnabled)
}

func (s *KymaService) GetKyma(ctx context.Context, namespacedName client.ObjectKey) (*v1beta2.Kyma, error) {
	return s.kymaRepository.Get(ctx, namespacedName)
}

func (s *KymaService) UpdateKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	return s.kymaRepository.Update(ctx, kyma)
}

func (r *KymaService) GetRelatedManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) ([]v1beta2.Manifest,
	error,
) {

	manifests, err := r.manifestRepository.ListByLabel(ctx, k8slabels.SelectorFromSet(k8slabels.Set{
		shared.KymaName: kyma.Name,
	}))

	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get related manifests, %w", err)
	}
	return manifests.Items, nil
}

func (r *KymaService) SyncKymaEnabled(kyma *v1beta2.Kyma) bool {
	if !r.inKCPMode {
		return false
	}
	return kyma.HasSyncLabelEnabled()
}

func (r *KymaService) UpdateStatus(ctx context.Context, kyma *v1beta2.Kyma, state shared.State, message string,
) error {
	if err := status.Helper(r.kymaRepository, r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *KymaService) IsKymaManaged() bool {
	return r.isManagedKyma
}

func (r *KymaService) UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma) {
	if err := r.metrics.UpdateAll(kyma); err != nil {
		if metrics.IsMissingMetricsAnnotationOrLabel(err) {
			r.Event.Warning(kyma, metricsError, err)
		}
		logf.FromContext(ctx).V(log.DebugLevel).Info(fmt.Sprintf("error occurred while updating all metrics: %s", err))
	}
}
