package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

func TestMaintenanceWindowMetrics(t *testing.T) {
	// Create a new instance of MaintenanceWindowMetrics
	maintenanceWindowMetrics := metrics.NewMaintenanceWindowMetrics()

	// Test recording a successful config read
	maintenanceWindowMetrics.RecordConfigReadSuccess(true)
	if err := testutil.CollectAndCompare(maintenanceWindowMetrics.ConfigReadSuccessGauge, strings.NewReader(`
		# HELP lifecycle_mgr_maintenance_window_config_read_success `+
		`Indicates whether the maintenance window configuration was read successfully (1 for success, 0 for failure)
		# TYPE lifecycle_mgr_maintenance_window_config_read_success gauge
		lifecycle_mgr_maintenance_window_config_read_success 1
	`)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}

	// Test recording a failed config read
	maintenanceWindowMetrics.RecordConfigReadSuccess(false)
	if err := testutil.CollectAndCompare(maintenanceWindowMetrics.ConfigReadSuccessGauge, strings.NewReader(`
		# HELP lifecycle_mgr_maintenance_window_config_read_success `+
		`Indicates whether the maintenance window configuration was read successfully (1 for success, 0 for failure)
		# TYPE lifecycle_mgr_maintenance_window_config_read_success gauge
		lifecycle_mgr_maintenance_window_config_read_success 0
	`)); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}
