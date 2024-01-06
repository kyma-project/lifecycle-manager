package metrics

import (
	"fmt"
	"testing"
)

func Test_ConstantMetricNames(t *testing.T) {
	tests := []struct {
		constName     string
		constValue    string
		expectedValue string
	}{
		{
			constName:     "MetricKymaState",
			constValue:    MetricKymaState,
			expectedValue: "lifecycle_mgr_kyma_state",
		},
		{
			constName:     "MetricModuleState",
			constValue:    MetricModuleState,
			expectedValue: "lifecycle_mgr_module_state",
		},
		{
			constName:     "metricPurgeTime",
			constValue:    metricPurgeTime,
			expectedValue: "lifecycle_mgr_purgectrl_time",
		},
		{
			constName:     "metricPurgeRequests",
			constValue:    metricPurgeRequests,
			expectedValue: "lifecycle_mgr_purgectrl_requests_total",
		},
		{
			constName:     "metricPurgeError",
			constValue:    metricPurgeError,
			expectedValue: "lifecycle_mgr_purgectrl_error",
		},
		{
			constName:     "SelfSignedCertNotRenewMetrics",
			constValue:    SelfSignedCertNotRenewMetrics,
			expectedValue: "lifecycle_mgr_self_signed_cert_not_renew",
		},
	}
	for _, tt := range tests {
		testName := fmt.Sprintf("const %s has default value", tt.constName)
		t.Run(testName, func(t *testing.T) {
			if tt.constValue != tt.expectedValue {
				t.Errorf("const %s does not have default value: expected = %s, got = %s",
					tt.constName, tt.expectedValue, tt.constValue)
			}
		})
	}
}
