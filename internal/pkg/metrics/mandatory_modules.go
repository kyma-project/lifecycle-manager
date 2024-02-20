package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricMandatoryTemplateCount = "lifecycle_mgr_mandatory_modules_count"
)

type MandatoryModulesMetrics struct {
	mandatoryModuleTemplatesCounter prometheus.Gauge
}

func NewMandatoryModulesMetrics() *MandatoryModulesMetrics {
	metrics := &MandatoryModulesMetrics{
		mandatoryModuleTemplatesCounter: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricMandatoryTemplateCount,
			Help: "Indicates the count of mandatory ModuleTemplates",
		}),
	}
	ctrlmetrics.Registry.MustRegister(metrics.mandatoryModuleTemplatesCounter)
	return metrics
}

func (m *MandatoryModulesMetrics) RecordMandatoryTemplatesCount(count int) {
	m.mandatoryModuleTemplatesCounter.Set(float64(count))
}
