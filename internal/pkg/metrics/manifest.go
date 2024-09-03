package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

type ManifestRequeueReason string

const (
	MetricManifestDuration                                     = "reconcile_duration_seconds"
	ManifestNameLabel                                          = "manifest_name"
	ManifestTypeCast                     ManifestRequeueReason = "manifest_type_cast"
	ManifestRetrieval                    ManifestRequeueReason = "manifest_retrieval"
	ManifestInit                         ManifestRequeueReason = "manifest_initialize"
	ManifestAddFinalizer                 ManifestRequeueReason = "manifest_add_finalizer"
	ManifestParseSpec                    ManifestRequeueReason = "manifest_parse_spec"
	ManifestUpdateSyncedOCIRef           ManifestRequeueReason = "manifest_update_synced_oci_ref"
	ManifestInitSyncedOCIRef             ManifestRequeueReason = "manifest_init_synced_oci_ref"
	ManifestClientInit                   ManifestRequeueReason = "manifest_client_init"
	ManifestRenderResources              ManifestRequeueReason = "manifest_render_resources"
	ManifestPruneDiffNotFinished         ManifestRequeueReason = "manifest_prune_diff_not_finished"
	ManifestPruneDiff                    ManifestRequeueReason = "manifest_prune_diff"
	ManifestPreDeleteEnqueueRequired     ManifestRequeueReason = "manifest_pre_delete_enqueue_required"
	ManifestPreDelete                    ManifestRequeueReason = "manifest_pre_delete"
	ManifestSyncResourcesEnqueueRequired ManifestRequeueReason = "manifest_sync_resources_enqueue_required"
	ManifestSyncResources                ManifestRequeueReason = "manifest_sync_resources"
	ManifestUnauthorized                 ManifestRequeueReason = "manifest_unauthorized"
	ManifestReconcileFinished            ManifestRequeueReason = "manifest_reconcile_finished"
	ManifestUnmanagedUpdate              ManifestRequeueReason = "manifest_unmanaged_update"
)

type ManifestMetrics struct {
	*SharedMetrics
	ManifestDurationGauge *prometheus.GaugeVec
}

func NewManifestMetrics(sharedMetrics *SharedMetrics) *ManifestMetrics {
	metrics := &ManifestMetrics{
		SharedMetrics: sharedMetrics,
		ManifestDurationGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricManifestDuration,
			Help: "Indicates the duration for manifest reconciliation in seconds",
		}, []string{ManifestNameLabel}),
	}

	ctrlmetrics.Registry.MustRegister(metrics.ManifestDurationGauge)
	return metrics
}

func (k *ManifestMetrics) RecordRequeueReason(requeueReason ManifestRequeueReason, requeueType queue.RequeueType) {
	k.requeueReasonCounter.WithLabelValues(string(requeueReason), string(requeueType)).Inc()
}

func (k *ManifestMetrics) RecordManifestDuration(manifestName string, duration time.Duration) {
	k.ManifestDurationGauge.WithLabelValues(manifestName).Set(duration.Seconds())
}

func (k *ManifestMetrics) RemoveManifestDuration(manifestName string) {
	k.ManifestDurationGauge.DeletePartialMatch(prometheus.Labels{
		ManifestNameLabel: manifestName,
	})
}
