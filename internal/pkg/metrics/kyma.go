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
	MetricRequeueReason = "lifecycle_mgr_requeue_reason_total"
	stateLabel          = "state"
	moduleNameLabel     = "module_name"
)

type KymaMetrics struct {
	kymaStateGauge   *prometheus.GaugeVec
	moduleStateGauge *prometheus.GaugeVec
	*SharedMetrics
}

type KymaRequeueReason string

const (
	//nolint:gosec // requeue reason label, no confidential content
	KymaUnderDeletionAndAccessSecretNotFound KymaRequeueReason = "kyma_under_deletion_with_no_access_secret"
	StatusUpdateToDeleting                   KymaRequeueReason = "kyma_status_update_to_deleting"
	LabelsAndFinalizersUpdate                KymaRequeueReason = "labels_and_finalizers_update"
	CrdAnnotationsUpdate                     KymaRequeueReason = "crd_annotations_update"
	CrdsSync                                 KymaRequeueReason = "crds_sync"
	ReconcileManifests                       KymaRequeueReason = "manifest_reconciliation"
	ModuleCatalogSync                        KymaRequeueReason = "module_catalog_sync"
	SkrWebhookResourcesInstallation          KymaRequeueReason = "skr_webhook_installation"
	InitialStateHandling                     KymaRequeueReason = "initial_state_handling"
	SyncContextRetrieval                     KymaRequeueReason = "sync_context_retrieval"
	RemoteKymaDeletion                       KymaRequeueReason = "remote_kyma_deletion"
	SpecReplacementFromRemote                KymaRequeueReason = "spec_replacement_from_remote"
	StatusSyncToRemote                       KymaRequeueReason = "status_sync_to_remote"
	ProcessingKymaState                      KymaRequeueReason = "processing_kyma_state"
	FinalizersRemovalFromRemoteKyma          KymaRequeueReason = "finalizers_removal_from_remote_kyma"
	RemoteModuleCatalogDeletion              KymaRequeueReason = "remote_module_catalog_deletion"
	CleanupManifestCrs                       KymaRequeueReason = "manifest_crs_cleanup"
	KymaDeletion                             KymaRequeueReason = "kyma_deletion"
	KymaRetrieval                            KymaRequeueReason = "kyma_retrieval"
)

func NewKymaMetrics(sharedMetrics *SharedMetrics) *KymaMetrics {
	kymaMetrics := &KymaMetrics{
		SharedMetrics: sharedMetrics,
		kymaStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricKymaState,
			Help: "Indicates the Status.state for a given Kyma object",
		}, []string{KymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),

		moduleStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricModuleState,
			Help: "Indicates the Status.state for modules of Kyma",
		}, []string{moduleNameLabel, KymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),
	}
	ctrlmetrics.Registry.MustRegister(kymaMetrics.kymaStateGauge)
	ctrlmetrics.Registry.MustRegister(kymaMetrics.moduleStateGauge)
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

func (k *KymaMetrics) RecordRequeueReason(kymaRequeueReason KymaRequeueReason, requeueType RequeueType) {
	k.requeueReasonCounter.WithLabelValues(string(kymaRequeueReason), string(requeueType)).Inc()
}
