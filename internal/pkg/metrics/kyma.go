package metrics

import (
	"context"
	"fmt"

	"github.com/cert-manager/cert-manager/pkg/logs"
	"github.com/prometheus/client_golang/prometheus"
	prometheusclient "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	KymaStateGauge   *prometheus.GaugeVec
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
		KymaStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricKymaState,
			Help: "Indicates the Status.state for a given Kyma object",
		}, []string{KymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),

		moduleStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricModuleState,
			Help: "Indicates the Status.state for modules of Kyma",
		}, []string{moduleNameLabel, KymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),
	}
	ctrlmetrics.Registry.MustRegister(kymaMetrics.KymaStateGauge)
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
	k.KymaStateGauge.DeletePartialMatch(prometheus.Labels{
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
		k.KymaStateGauge.With(prometheus.Labels{
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

func (k *KymaMetrics) CleanupNonExistingKymaCrsMetrics(ctx context.Context, kcpClient client.Client) error {
	currentLifecycleManagerMetrics, err := FetchLifecycleManagerMetrics()
	if err != nil {
		return err
	}

	if len(currentLifecycleManagerMetrics) == 0 {
		return nil
	}

	kymaCrsList := &v1beta2.KymaList{}
	err = kcpClient.List(ctx, kymaCrsList)
	if err != nil {
		return fmt.Errorf("failed to fetch Kyma CRs, %w", err)
	}
	kymaNames := getKymaNames(kymaCrsList)
	for _, m := range currentLifecycleManagerMetrics {
		currentKymaName := getKymaNameFromLabels(m)
		if currentKymaName == "" {
			continue
		}
		if _, exists := kymaNames[currentKymaName]; !exists {
			logs.FromContext(ctx).Info(fmt.Sprintf("Deleting a metric for non-existing Kyma: %s", currentKymaName))
			k.KymaStateGauge.DeletePartialMatch(prometheus.Labels{
				KymaNameLabel: currentKymaName,
			})
		}
	}

	return nil
}

func FetchLifecycleManagerMetrics() ([]*prometheusclient.Metric, error) {
	currentMetrics, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current kyma metrics, %w", err)
	}

	for _, metric := range currentMetrics {
		if metric.GetName() == MetricKymaState {
			return metric.GetMetric(), nil
		}
	}

	return nil, nil
}

func getKymaNameFromLabels(metric *prometheusclient.Metric) string {
	for _, label := range metric.GetLabel() {
		if label.GetName() == KymaNameLabel {
			return label.GetValue()
		}
	}

	return ""
}

func getKymaNames(kymaCrs *v1beta2.KymaList) map[string]bool {
	if len(kymaCrs.Items) == 0 {
		return nil
	}

	names := make(map[string]bool)
	for _, kyma := range kymaCrs.Items {
		names[kyma.GetName()] = true
	}
	return names
}
