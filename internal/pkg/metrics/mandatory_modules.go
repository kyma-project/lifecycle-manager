package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	MetricMandatoryTemplateCount = "lifecycle_mgr_mandatory_modules"
	MetricMandatoryModuleState   = "lifecycle_mgr_mandatory_module_state"
)

type MandatoryModulesMetrics struct {
	mandatoryModuleTemplatesCounter prometheus.Gauge
	moduleStateGauge                *prometheus.GaugeVec
}

func NewMandatoryModulesMetrics() *MandatoryModulesMetrics {
	metrics := &MandatoryModulesMetrics{
		mandatoryModuleTemplatesCounter: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricMandatoryTemplateCount,
			Help: "Indicates the count of mandatory ModuleTemplates",
		}),
		moduleStateGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricMandatoryModuleState,
			Help: "Indicates the Status.state for mandatory modules of Kyma",
		}, []string{moduleNameLabel, KymaNameLabel, stateLabel}),
	}
	ctrlmetrics.Registry.MustRegister(metrics.mandatoryModuleTemplatesCounter)
	return metrics
}

func (m *MandatoryModulesMetrics) RecordMandatoryTemplatesCount(count int) {
	m.mandatoryModuleTemplatesCounter.Set(float64(count))
}

func (m *MandatoryModulesMetrics) RecordMandatoryModuleState(kymaName, moduleName string, newState shared.State) {
	states := shared.AllStates()
	for _, state := range states {
		newValue := calcStateValue(state, newState)
		m.moduleStateGauge.With(prometheus.Labels{
			KymaNameLabel:   kymaName,
			moduleNameLabel: moduleName,
			stateLabel:      string(state),
		}).Set(newValue)
	}
}
