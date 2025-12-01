package deletion

import (
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
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
	case usecase.SetKcpKymaStateDeleting:
		w.metrics.RecordRequeueReason(metrics.StatusUpdateToDeleting, requeueType)
	case usecase.SetSkrKymaStateDeleting:
		w.metrics.RecordRequeueReason(metrics.StatusSyncToRemote, requeueType)
	case usecase.DeleteSkrKyma:
		w.metrics.RecordRequeueReason(metrics.RemoteKymaDeletion, requeueType)
	case usecase.DeleteSkrModuleTemplateCrd, usecase.DeleteSkrModuleReleaseMetaCrd:
		w.metrics.RecordRequeueReason(metrics.RemoteModuleCatalogDeletion, requeueType)
	case usecase.DeleteManifests:
		w.metrics.RecordRequeueReason(metrics.CleanupManifestCrs, requeueType)
	case usecase.DeleteSkrWatcher,
		usecase.DeleteSkrKymaCrd,
		usecase.DeleteWatcherCertificate,
		usecase.DeleteMetrics,
		usecase.RemoveKymaFinalizers:
		// These use cases are not tracked by metrics.
	}
}
