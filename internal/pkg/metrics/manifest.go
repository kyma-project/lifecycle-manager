package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

type ManifestRequeueReason string

const (
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
)

type ManifestMetrics struct {
	requeueReasonCounter *prometheus.CounterVec
}

func NewManifestMetrics() *ManifestMetrics {
	metrics := &ManifestMetrics{
		requeueReasonCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: MetricRequeueReason,
			Help: "Indicates the reason for manifest reconciliation requeue",
		}, []string{requeueReasonLabel, requeueTypeLabel}),
	}
	ctrlmetrics.Registry.MustRegister(metrics.requeueReasonCounter)
	return metrics
}

func (k *ManifestMetrics) RecordRequeueReason(requeueReason ManifestRequeueReason, requeueType RequeueType) {
	k.requeueReasonCounter.WithLabelValues(string(requeueReason), string(requeueType)).Inc()
}
