package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

type ManifestRequeueReason string

const (
	metricReconcileDuration               string                = "reconcile_duration_seconds"
	metricLabelModule                     string                = "module_name"
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
				Name:    metricReconcileDuration,
				Help:    "Histogram of reconcile duration for manifest reconciliation in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{metricLabelModule},
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
