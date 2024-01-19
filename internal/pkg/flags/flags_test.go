package flags_test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

func Test_ConstantFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		constName     string
		constValue    string
		expectedValue string
	}{
		{
			constName:     "DefaultKymaRequeueSuccessInterval",
			constValue:    string(DefaultKymaRequeueSuccessInterval),
			expectedValue: string(30 * time.Second),
		},
		{
			constName:     "DefaultKymaRequeueErrInterval",
			constValue:    string(DefaultKymaRequeueErrInterval),
			expectedValue: string(2 * time.Second),
		},
		{
			constName:     "DefaultKymaRequeueWarningInterval",
			constValue:    string(DefaultKymaRequeueWarningInterval),
			expectedValue: string(30 * time.Second),
		},
		{
			constName:     "DefaultKymaRequeueBusyInterval",
			constValue:    string(DefaultKymaRequeueBusyInterval),
			expectedValue: string(5 * time.Second),
		},
		{
			constName:     "DefaultManifestRequeueSuccessInterval",
			constValue:    string(DefaultManifestRequeueSuccessInterval),
			expectedValue: string(30 * time.Second),
		}, {
			constName:     "DefaultMandatoryModuleRequeueSuccessInterval",
			constValue:    string(DefaultMandatoryModuleRequeueSuccessInterval),
			expectedValue: string(30 * time.Second),
		},
		{
			constName:     "DefaultWatcherRequeueSuccessInterval",
			constValue:    string(DefaultWatcherRequeueSuccessInterval),
			expectedValue: string(30 * time.Second),
		},
		{
			constName:     "DefaultClientQPS",
			constValue:    string(DefaultClientQPS),
			expectedValue: "300",
		},
		{
			constName:     "DefaultClientBurst",
			constValue:    string(DefaultClientBurst),
			expectedValue: "600",
		},
		{
			constName:     "DefaultPprofServerTimeout",
			constValue:    string(DefaultPprofServerTimeout),
			expectedValue: string(90 * time.Second),
		},
		{
			constName:     "RateLimiterBurstDefault",
			constValue:    string(RateLimiterBurstDefault),
			expectedValue: "200",
		},
		{
			constName:     "RateLimiterFrequencyDefault",
			constValue:    string(RateLimiterFrequencyDefault),
			expectedValue: "30",
		},
		{
			constName:     "FailureBaseDelayDefault",
			constValue:    string(FailureBaseDelayDefault),
			expectedValue: string(100 * time.Millisecond),
		},
		{
			constName:     "FailureMaxDelayDefault",
			constValue:    string(FailureMaxDelayDefault),
			expectedValue: string(5 * time.Second),
		},
		{
			constName:     "DefaultCacheSyncTimeout",
			constValue:    string(DefaultCacheSyncTimeout),
			expectedValue: string(2 * time.Minute),
		},
		{
			constName:     "DefaultLogLevel",
			constValue:    string(DefaultLogLevel),
			expectedValue: string(log.WarnLevel),
		},
		{
			constName:     "DefaultPurgeFinalizerTimeout",
			constValue:    string(DefaultPurgeFinalizerTimeout),
			expectedValue: string(5 * time.Minute),
		},
		{
			constName:     "DefaultMaxConcurrentManifestReconciles",
			constValue:    string(DefaultMaxConcurrentManifestReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultMaxConcurrentKymaReconciles",
			constValue:    string(DefaultMaxConcurrentKymaReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultMaxConcurrentWatcherReconciles",
			constValue:    string(DefaultMaxConcurrentWatcherReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultMaxConcurrentMandatoryModulesReconciles",
			constValue:    string(DefaultMaxConcurrentMandatoryModulesReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultRemoteSyncNamespace",
			constValue:    DefaultRemoteSyncNamespace,
			expectedValue: "kyma-system",
		},
		{
			constName:     "DefaultIstioGatewayName",
			constValue:    DefaultIstioGatewayName,
			expectedValue: "klm-watcher-gateway",
		},
		{
			constName:     "DefaultIstioGatewayNamespace",
			constValue:    DefaultIstioGatewayNamespace,
			expectedValue: "kcp-system",
		},
		{
			constName:     "DefaultIstioNamespace",
			constValue:    DefaultIstioNamespace,
			expectedValue: "istio-system",
		},
		{
			constName:     "DefaultCaCertName",
			constValue:    DefaultCaCertName,
			expectedValue: "klm-watcher-serving-cert",
		},
		{
			constName:     "DefaultCaCertCacheTTL",
			constValue:    string(DefaultCaCertCacheTTL),
			expectedValue: string(1 * time.Hour),
		},
		{
			constName:     "DefaultSelfSignedCertDuration",
			constValue:    string(DefaultSelfSignedCertDuration),
			expectedValue: string(90 * 24 * time.Hour),
		},
		{
			constName:     "DefaultSelfSignedCertRenewBefore",
			constValue:    string(DefaultSelfSignedCertRenewBefore),
			expectedValue: string(90 * 24 * time.Hour),
		},
		{
			constName:     "DefaultSelfSignedCertificateRenewBuffer",
			constValue:    string(DefaultSelfSignedCertificateRenewBuffer),
			expectedValue: string(24 * time.Hour),
		},
	}
	for _, testcase := range tests {
		testcase := testcase
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
