package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricMaintenanceWindowConfigReadSuccess = "lifecycle_mgr_maintenance_window_config_read_success"
)

type MaintenanceWindowMetrics struct {
	ConfigReadSuccessGauge prometheus.Gauge
}

func NewMaintenanceWindowMetrics() *MaintenanceWindowMetrics {
	metrics := &MaintenanceWindowMetrics{
		ConfigReadSuccessGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: MetricMaintenanceWindowConfigReadSuccess,
			Help: "Indicates whether the maintenance window configuration " +
				"was read successfully (1 for success, 0 for failure)",
		}),
	}
	ctrlmetrics.Registry.MustRegister(metrics.ConfigReadSuccessGauge)
	return metrics
}

func (m *MaintenanceWindowMetrics) RecordConfigReadSuccess(success bool) {
	value := 0.0
	if success {
		value = 1.0
	}
	m.ConfigReadSuccessGauge.Set(value)
}
