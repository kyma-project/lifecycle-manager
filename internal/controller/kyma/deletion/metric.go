package deletion

import (
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	resultkymadeletion "github.com/kyma-project/lifecycle-manager/internal/result/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

type MetricRepo interface {
	RecordRequeueReason(reason metrics.KymaRequeueReason, requeueType queue.RequeueType)
}

type MetricWriter struct {
	metrics MetricRepo
}

func NewMetricWriter(metricsRepo MetricRepo) *MetricWriter {
	return &MetricWriter{
		metrics: metricsRepo,
	}
}

// Write records the Kyma requeue reason metric based on the result's use case.
// If the result contains an error, it classifies the requeue as unexpected; otherwise, it's intended.
func (w *MetricWriter) Write(res result.Result) {
	requeueType := queue.IntendedRequeue

	if res.Err != nil {
		requeueType = queue.UnexpectedRequeue
	}

	switch res.UseCase {
	case resultkymadeletion.UseCaseSetKcpKymaStateDeleting:
		w.metrics.RecordRequeueReason(metrics.StatusUpdateToDeleting, requeueType)
	case resultkymadeletion.UseCaseSetSkrKymaStateDeleting:
		w.metrics.RecordRequeueReason(metrics.StatusSyncToRemote, requeueType)
	case resultkymadeletion.UseCaseDeleteSkrKyma:
		w.metrics.RecordRequeueReason(metrics.RemoteKymaDeletion, requeueType)
	case resultkymadeletion.UseCaseDeleteSkrModuleMetadata:
		w.metrics.RecordRequeueReason(metrics.RemoteModuleCatalogDeletion, requeueType)
	case resultkymadeletion.UseCaseDeleteManifests:
		w.metrics.RecordRequeueReason(metrics.CleanupManifestCrs, requeueType)
	case resultkymadeletion.UseCaseDeleteSkrWatcher,
		resultkymadeletion.UseCaseDeleteSkrCrds,
		resultkymadeletion.UseCaseDeleteWatcherCertificate,
		resultkymadeletion.UseCaseDeleteMetrics,
		resultkymadeletion.UseCaseRemoveKymaFinalizers:
		// These use cases are not tracked by metrics.
	}
}
