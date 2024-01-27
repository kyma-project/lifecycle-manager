package flags_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/log"

	. "github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
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
			constValue:    DefaultKymaRequeueSuccessInterval.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultKymaRequeueErrInterval",
			constValue:    DefaultKymaRequeueErrInterval.String(),
			expectedValue: (2 * time.Second).String(),
		},
		{
			constName:     "DefaultKymaRequeueWarningInterval",
			constValue:    DefaultKymaRequeueWarningInterval.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultKymaRequeueBusyInterval",
			constValue:    DefaultKymaRequeueBusyInterval.String(),
			expectedValue: (5 * time.Second).String(),
		},
		{
			constName:     "DefaultManifestRequeueSuccessInterval",
			constValue:    DefaultManifestRequeueSuccessInterval.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultMandatoryModuleRequeueSuccessInterval",
			constValue:    DefaultMandatoryModuleRequeueSuccessInterval.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultMandatoryModuleDeletionRequeueSuccessInterval",
			constValue:    DefaultMandatoryModuleDeletionRequeueSuccessInterval.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultWatcherRequeueSuccessInterval",
			constValue:    DefaultWatcherRequeueSuccessInterval.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultClientQPS",
			constValue:    strconv.Itoa(DefaultClientQPS),
			expectedValue: "300",
		},
		{
			constName:     "DefaultClientBurst",
			constValue:    strconv.Itoa(DefaultClientBurst),
			expectedValue: "600",
		},
		{
			constName:     "DefaultPprofServerTimeout",
			constValue:    DefaultPprofServerTimeout.String(),
			expectedValue: (90 * time.Second).String(),
		},
		{
			constName:     "RateLimiterBurstDefault",
			constValue:    strconv.Itoa(RateLimiterBurstDefault),
			expectedValue: "200",
		},
		{
			constName:     "RateLimiterFrequencyDefault",
			constValue:    strconv.Itoa(RateLimiterFrequencyDefault),
			expectedValue: "30",
		},
		{
			constName:     "FailureBaseDelayDefault",
			constValue:    FailureBaseDelayDefault.String(),
			expectedValue: (100 * time.Millisecond).String(),
		},
		{
			constName:     "FailureMaxDelayDefault",
			constValue:    FailureMaxDelayDefault.String(),
			expectedValue: (5 * time.Second).String(),
		},
		{
			constName:     "DefaultCacheSyncTimeout",
			constValue:    DefaultCacheSyncTimeout.String(),
			expectedValue: (2 * time.Minute).String(),
		},
		{
			constName:     "DefaultLogLevel",
			constValue:    strconv.Itoa(DefaultLogLevel),
			expectedValue: strconv.Itoa(log.WarnLevel),
		},
		{
			constName:     "DefaultPurgeFinalizerTimeout",
			constValue:    DefaultPurgeFinalizerTimeout.String(),
			expectedValue: (5 * time.Minute).String(),
		},
		{
			constName:     "DefaultMaxConcurrentManifestReconciles",
			constValue:    strconv.Itoa(DefaultMaxConcurrentManifestReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultMaxConcurrentKymaReconciles",
			constValue:    strconv.Itoa(DefaultMaxConcurrentKymaReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultMaxConcurrentWatcherReconciles",
			constValue:    strconv.Itoa(DefaultMaxConcurrentWatcherReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultMaxConcurrentMandatoryModuleDeletionReconciles",
			constValue:    strconv.Itoa(DefaultMaxConcurrentMandatoryModuleDeletionReconciles),
			expectedValue: "1",
		},
		{
			constName:     "DefaultMaxConcurrentMandatoryModuleReconciles",
			constValue:    strconv.Itoa(DefaultMaxConcurrentMandatoryModuleReconciles),
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
			constValue:    DefaultCaCertCacheTTL.String(),
			expectedValue: (1 * time.Hour).String(),
		},
		{
			constName:     "DefaultSelfSignedCertDuration",
			constValue:    DefaultSelfSignedCertDuration.String(),
			expectedValue: (90 * 24 * time.Hour).String(),
		},
		{
			constName:     "DefaultSelfSignedCertRenewBefore",
			constValue:    DefaultSelfSignedCertRenewBefore.String(),
			expectedValue: (60 * 24 * time.Hour).String(),
		},
		{
			constName:     "DefaultSelfSignedCertificateRenewBuffer",
			constValue:    DefaultSelfSignedCertificateRenewBuffer.String(),
			expectedValue: (24 * time.Hour).String(),
		},
		{
			constName:     "DefaultMetricsAddress",
			constValue:    DefaultMetricsAddress,
			expectedValue: ":8080",
		},
		{
			constName:     "DefaultProbeAddress",
			constValue:    DefaultProbeAddress,
			expectedValue: ":8081",
		},
		{
			constName:     "DefaultKymaListenerAddress",
			constValue:    DefaultKymaListenerAddress,
			expectedValue: ":8082",
		},
		{
			constName:     "DefaultManifestListenerAddr",
			constValue:    DefaultManifestListenerAddress,
			expectedValue: ":8083",
		},
		{
			constName:     "DefaultPprofAddress",
			constValue:    DefaultPprofAddress,
			expectedValue: ":8084",
		},
		{
			constName:     "DefaultWatcherResourcesPath",
			constValue:    DefaultWatcherResourcesPath,
			expectedValue: "./skr-webhook",
		},
		{
			constName:     "DefaultWatcherResourceLimitsCPU",
			constValue:    DefaultWatcherResourceLimitsCPU,
			expectedValue: "0.1",
		},
		{
			constName:     "DefaultWatcherResourceLimitsMemory",
			constValue:    DefaultWatcherResourceLimitsMemory,
			expectedValue: "200Mi",
		},
		{
			constName:     "DefaultDropStoredVersion",
			constValue:    DefaultDropStoredVersion,
			expectedValue: "v1alpha1",
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
