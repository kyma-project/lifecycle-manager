package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

type ManifestRequeueReason string

const (
	MetricReconcileDuration               string                = "reconcile_duration_seconds"
	MetricLabelModule                     string                = "module_name"
	ManifestTypeCast                      ManifestRequeueReason = "manifest_type_cast"
	ManifestRetrieval                     ManifestRequeueReason = "manifest_retrieval"
	ManifestInit                          ManifestRequeueReason = "manifest_initialize"
	ManifestAddFinalizer                  ManifestRequeueReason = "manifest_add_finalizer"
	ManifestParseSpec                     ManifestRequeueReason = "manifest_parse_spec"
	ManifestUpdateSyncedOCIRef            ManifestRequeueReason = "manifest_update_synced_oci_ref"
	ManifestInitSyncedOCIRef              ManifestRequeueReason = "manifest_init_synced_oci_ref"
	ManifestRemoveFinalizerInDeleting     ManifestRequeueReason = "manifest_remove_finalizer_in_deleting"
	ManifestRemoveFinalizerWhenParseSpec  ManifestRequeueReason = "manifest_remove_finalizer_when_parse_spec"
	ManifestRemoveFinalizerWhenSecretGone ManifestRequeueReason = "manifest_remove_finalizer_when_secret_gone"
	ManifestClientInit                    ManifestRequeueReason = "manifest_client_init"
	ManifestRenderResources               ManifestRequeueReason = "manifest_render_resources"
	ManifestPruneDiffNotFinished          ManifestRequeueReason = "manifest_prune_diff_not_finished"
	ManifestPruneDiff                     ManifestRequeueReason = "manifest_prune_diff"
	ManifestPreDeleteEnqueueRequired      ManifestRequeueReason = "manifest_pre_delete_enqueue_required"
	ManifestPreDelete                     ManifestRequeueReason = "manifest_pre_delete"
	ManifestSyncResourcesEnqueueRequired  ManifestRequeueReason = "manifest_sync_resources_enqueue_required"
	ManifestSyncResources                 ManifestRequeueReason = "manifest_sync_resources"
	ManifestUnauthorized                  ManifestRequeueReason = "manifest_unauthorized"
)

type ManifestMetrics struct {
	*SharedMetrics
	reconcileDurationHistogram *prometheus.HistogramVec
}

func NewManifestMetrics(sharedMetrics *SharedMetrics) *ManifestMetrics {
	metrics := &ManifestMetrics{
		SharedMetrics: sharedMetrics,
		reconcileDurationHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: MetricReconcileDuration,
				Help: "Histogram of reconcile duration for manifest reconciliation in seconds",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.6,
					0.7, 0.8, 0.9, 1.0, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5, 6, 7, 8, 9, 10, 15, 20,
					25, 30, 40, 50, 60},
			},
			[]string{MetricLabelModule},
		),
	}
	ctrlmetrics.Registry.MustRegister(metrics.reconcileDurationHistogram)
	return metrics
}

func (k *ManifestMetrics) RecordRequeueReason(requeueReason ManifestRequeueReason, requeueType queue.RequeueType) {
	k.requeueReasonCounter.WithLabelValues(string(requeueReason), string(requeueType)).Inc()
}

func (k *ManifestMetrics) ObserveReconcileDuration(reconcileStart time.Time, moduleName string) {
	durationInSeconds := time.Since(reconcileStart).Seconds()
	k.reconcileDurationHistogram.WithLabelValues(moduleName).
		Observe(durationInSeconds)
}
