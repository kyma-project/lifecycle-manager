package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	metricKymaState   = "lifecycle_mgr_kyma_state"
	metricModuleState = "lifecycle_mgr_module_state"
	stateLabel        = "state"
	moduleNameLabel   = "module_name"
)

type KymaMetrics struct {
	kymaStateGauge   *prometheus.GaugeVec
	moduleStateGauge *prometheus.GaugeVec
}

func NewKymaMetrics() *KymaMetrics {
	kymaMetrics := &KymaMetrics{
		kymaStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricKymaState,
			Help: "Indicates the Status.state for a given Kyma object",
		}, []string{kymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),

		moduleStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricModuleState,
			Help: "Indicates the Status.state for modules of Kyma",
		}, []string{moduleNameLabel, kymaNameLabel, stateLabel, shootIDLabel, instanceIDLabel}),
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
		kymaNameLabel: kymaName,
	})
	k.moduleStateGauge.DeletePartialMatch(prometheus.Labels{
		kymaNameLabel: kymaName,
	})
}

// RemoveModuleStateMetrics deletes all 'lifecycle_mgr_module_state' metrics for the matching module.
func (k *KymaMetrics) RemoveModuleStateMetrics(kyma *v1beta2.Kyma, moduleName string) {
	k.moduleStateGauge.DeletePartialMatch(prometheus.Labels{
		moduleNameLabel: moduleName,
		kymaNameLabel:   kyma.Name,
	})
}

func (k *KymaMetrics) setKymaStateGauge(newState shared.State, kymaName, shootID, instanceID string) {
	states := shared.AllStates()
	for _, state := range states {
		newValue := calcStateValue(state, newState)
		k.kymaStateGauge.With(prometheus.Labels{
			kymaNameLabel:   kymaName,
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
			kymaNameLabel:   kymaName,
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
