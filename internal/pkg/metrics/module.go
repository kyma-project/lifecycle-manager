package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

type ModuleCondition string

const (
	MetricModuleCondition                 = "lifecycle_mgr_module_condition"
	conditionLabel                        = "condition"
	moduleCrWarning       ModuleCondition = "ModuleCRWarning"
)

type ModuleMetrics struct {
	moduleCRConditionGauge *prometheus.GaugeVec
}

func NewModuleMetrics() *ModuleMetrics {
	metrics := &ModuleMetrics{
		moduleCRConditionGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricModuleCondition,
			Help: "Manifest is in Deleting state and related ModuleCR is in Warning state, indicating that Deletion is blocked and requires user interaction.",
		}, []string{KymaNameLabel, moduleNameLabel, conditionLabel}),
	}
	ctrlmetrics.Registry.MustRegister(metrics.moduleCRConditionGauge)
	return metrics
}

func (m *ModuleMetrics) SetModuleCRWarningCondition(kymaName, moduleName string) {
	m.moduleCRConditionGauge.With(prometheus.Labels{
		KymaNameLabel:  kymaName,
		moduleName:     moduleName,
		conditionLabel: string(moduleCrWarning),
	}).Set(1)
}

func (m *ModuleMetrics) RemoveModuleCRWarningCondition(kymaName, moduleName string) {
	m.moduleCRConditionGauge.DeletePartialMatch(prometheus.Labels{
		KymaNameLabel:   kymaName,
		moduleNameLabel: moduleName,
		conditionLabel:  string(moduleCrWarning),
	})
}

func (m *ModuleMetrics) CleanupMetrics(kymaName, moduleName string) {
	m.moduleCRConditionGauge.DeletePartialMatch(prometheus.Labels{
		KymaNameLabel:   kymaName,
		moduleNameLabel: moduleName,
	})
}
