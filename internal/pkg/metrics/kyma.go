package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	MetricKymaState     = "lifecycle_mgr_kyma_state"
	MetricModuleState   = "lifecycle_mgr_module_state"
	MetricRequeueReason = "lifecycle_mgr_requeue_reason"
	stateLabel          = "state"
	moduleNameLabel     = "module_name"
	requeueReasonLabel  = "requeue_reason"
)

type KymaMetrics struct {
	kymaStateGauge       *prometheus.GaugeVec
	moduleStateGauge     *prometheus.GaugeVec
	requeueReasonCounter *prometheus.CounterVec
}

type RequeueReason string

const (
	KymaRetrievalError                       RequeueReason = "kyma_retrieval_error"
	KymaUnderDeletionAndAccessSecretNotFound RequeueReason = "kyma_under_deletion_with_no_access_secret"
	StatusUpdateToDeleting                   RequeueReason = "kyma_status_update_to_deleting"
	LabelsAndFinalizersUpdate                RequeueReason = "labels_and_finalizers_update"
	LabelsAndFinalizersUpdateError           RequeueReason = "labels_and_finalizers_update_error"
	CrdAnnotationsUpdate                     RequeueReason = "crd_annotations_update"
	CrdAnnotationsUpdateError                RequeueReason = "crd_annotations_update_error"
	CrdsSyncError                            RequeueReason = "crds_sync_error"
	ManifestReconciliationError              RequeueReason = "manifest_reconciliation_error"
	ModuleCatalogSyncError                   RequeueReason = "module_catalog_sync_error"
	SkrWebhookResourcesInstallationError     RequeueReason = "skr_webhook_installation_error"
	InitialStateHandling                     RequeueReason = "initial_state_handling"
	InitialStateHandlingError                RequeueReason = "initial_state_handling_error"
	SyncContextRetrievalError                RequeueReason = "sync_context_retrieval_error"
	RemoteKymaDeletionError                  RequeueReason = "remote_kyma_deletion_error"
	StatusUpdateToDeletingError              RequeueReason = "status_update_to_deleting_error"
	SpecReplacementFromRemoteError           RequeueReason = "spec_replacement_from_remote_error"
	StatusSyncToRemoteError                  RequeueReason = "status_sync_to_remote_error"
	ProcessingKymaStateError                 RequeueReason = "processing_kyma_state_error"
	FinalizersRemovalFromRemoteKymaError     RequeueReason = "finalizers_removal_from_remote_kyma_error"
	RemoteModuleCatalogDeletionError         RequeueReason = "remote_module_catalog_deletion_error"
	ManifestCrsCleanupError                  RequeueReason = "manifest_crs_cleanup_error"
	KymaDeletion                             RequeueReason = "kyma_deletion"
)

func NewKymaMetrics() *KymaMetrics {
	kymaMetrics := &KymaMetrics{
		kymaStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricKymaState,
			Help: "Indicates the Status.state for a given Kyma object",
		}, []string{KymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),

		moduleStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricModuleState,
			Help: "Indicates the Status.state for modules of Kyma",
		}, []string{moduleNameLabel, KymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),

		requeueReasonCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: MetricRequeueReason,
			Help: "Indicates the reason for the Kyma reconciliation requeue",
		}, []string{requeueReasonLabel}),
	}
	ctrlmetrics.Registry.MustRegister(kymaMetrics.kymaStateGauge)
	ctrlmetrics.Registry.MustRegister(kymaMetrics.moduleStateGauge)
	ctrlmetrics.Registry.MustRegister(kymaMetrics.requeueReasonCounter)
	return kymaMetrics
}

// UpdateAll sets both metrics 'lifecycle_mgr_kyma_state' and 'lifecycle_mgr_module_state' to new states.
func (k *KymaMetrics) UpdateAll(kyma *v1beta2.Kyma) error {
	shootID, err := ExtractShootID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}
	instanceID, err := ExtractInstanceID(kyma)
	if err != nil {
		return fmt.Errorf("%w: %w", errMetric, err)
	}

	k.setKymaStateGauge(kyma.Status.State, kyma.Name, shootID, instanceID)
	for _, moduleStatus := range kyma.Status.Modules {
		k.setModuleStateGauge(moduleStatus.State, moduleStatus.Name, kyma.Name, shootID, instanceID)
	}
	return nil
}

// CleanupMetrics deletes all 'lifecycle_mgr_kyma_state',
// 'lifecycle_mgr_module_state' metrics for the matching Kyma.
func (k *KymaMetrics) CleanupMetrics(kymaName string) {
	k.kymaStateGauge.DeletePartialMatch(prometheus.Labels{
		KymaNameLabel: kymaName,
	})
	k.moduleStateGauge.DeletePartialMatch(prometheus.Labels{
		KymaNameLabel: kymaName,
	})
}

// RemoveModuleStateMetrics deletes all 'lifecycle_mgr_module_state' metrics for the matching module.
func (k *KymaMetrics) RemoveModuleStateMetrics(kymaName, moduleName string) {
	k.moduleStateGauge.DeletePartialMatch(prometheus.Labels{
		moduleNameLabel: moduleName,
		KymaNameLabel:   kymaName,
	})
}

func (k *KymaMetrics) setKymaStateGauge(newState shared.State, kymaName, shootID, instanceID string) {
	states := shared.AllStates()
	for _, state := range states {
		newValue := calcStateValue(state, newState)
		k.kymaStateGauge.With(prometheus.Labels{
			KymaNameLabel:   kymaName,
			shootIDLabel:    shootID,
			instanceIDLabel: instanceID,
			stateLabel:      string(state),
		}).Set(newValue)
	}
}

func (k *KymaMetrics) setModuleStateGauge(newState shared.State, moduleName, kymaName, shootID, instanceID string) {
	states := shared.AllStates()
	for _, state := range states {
		newValue := calcStateValue(state, newState)
		k.moduleStateGauge.With(prometheus.Labels{
			moduleNameLabel: moduleName,
			KymaNameLabel:   kymaName,
			shootIDLabel:    shootID,
			instanceIDLabel: instanceID,
			stateLabel:      string(state),
		}).Set(newValue)
	}
}

func calcStateValue(state, newState shared.State) float64 {
	if state == newState {
		return 1
	}
	return 0
}

func (k *KymaMetrics) RecordRequeueReason(requeueReason RequeueReason) {
	k.requeueReasonCounter.WithLabelValues(string(requeueReason)).Inc()
}
