package metrics_test

import (
	"strings"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMaintenanceWindowMetrics(t *testing.T) {
	// Create a new instance of MaintenanceWindowMetrics
	m := metrics.NewMaintenanceWindowMetrics()

	// Test recording a successful config read
	m.RecordConfigReadSuccess(true)
	if err := testutil.CollectAndCompare(m.ConfigReadSuccessGauge, strings.NewReader(`
		# HELP maintenance_window_config_read_success Indicates whether the maintenance window configuration was read successfully (1 for success, 0 for failure)
		# TYPE maintenance_window_config_read_success gauge
		maintenance_window_config_read_success 1
	`)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	// Test recording a failed config read
	m.RecordConfigReadSuccess(false)
	if err := testutil.CollectAndCompare(m.ConfigReadSuccessGauge, strings.NewReader(`
		# HELP maintenance_window_config_read_success Indicates whether the maintenance window configuration was read successfully (1 for success, 0 for failure)
		# TYPE maintenance_window_config_read_success gauge
		maintenance_window_config_read_success 0
	`)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
