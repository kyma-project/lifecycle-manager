package metrics_test

import (
	"fmt"
	"testing"

	. "github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
)

func Test_ConstantMetricNames(t *testing.T) {
	t.Parallel()
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
			constName:     "MetricPurgeTime",
			constValue:    MetricPurgeTime,
			expectedValue: "lifecycle_mgr_purgectrl_time",
		},
		{
			constName:     "MetricPurgeRequests",
			constValue:    MetricPurgeRequests,
			expectedValue: "lifecycle_mgr_purgectrl_requests_total",
		},
		{
			constName:     "MetricPurgeError",
			constValue:    MetricPurgeError,
			expectedValue: "lifecycle_mgr_purgectrl_error",
		},
		{
			constName:     "MetricSelfSignedCertNotRenew",
			constValue:    MetricSelfSignedCertNotRenew,
			expectedValue: "lifecycle_mgr_self_signed_cert_not_renew",
		},
		{
			constName:     "MetricManifestDuration",
			constValue:    MetricManifestDuration,
			expectedValue: "reconcile_duration_seconds",
		},
		{
			constName:     "MetricMandatoryTemplateCount",
			constValue:    MetricMandatoryTemplateCount,
			expectedValue: "lifecycle_mgr_mandatory_modules",
		},
		{
			constName:     "MetricMandatoryModuleState",
			constValue:    MetricMandatoryModuleState,
			expectedValue: "lifecycle_mgr_mandatory_module_state",
		},
	}
	for _, testcase := range tests {
		testName := fmt.Sprintf("const %s has correct value", testcase.constName)
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			if testcase.constValue != testcase.expectedValue {
				t.Errorf("const %s value is incorrect: expected = %s, got = %s",
					testcase.constName, testcase.expectedValue, testcase.constValue)
			}
		})
	}
}
