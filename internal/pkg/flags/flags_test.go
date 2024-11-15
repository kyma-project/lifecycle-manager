package flags_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
			constName:     "DefaultManifestRequeueErrInterval",
			constValue:    DefaultManifestRequeueErrInterval.String(),
			expectedValue: (2 * time.Second).String(),
		},
		{
			constName:     "DefaultManifestRequeueWarningInterval",
			constValue:    DefaultManifestRequeueWarningInterval.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultManifestRequeueBusyInterval",
			constValue:    DefaultManifestRequeueBusyInterval.String(),
			expectedValue: (5 * time.Second).String(),
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
			constName:     "DefaultRootCASecretName",
			constValue:    DefaultRootCASecretName,
			expectedValue: "klm-watcher",
		},
		{
			constName:     "DefaultRootCASecretNamespace",
			constValue:    DefaultRootCASecretNamespace,
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
			expectedValue: "klm-watcher-serving",
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
			constName:     "DefaultSelfSignedCertKeySize",
			constValue:    strconv.Itoa(int(DefaultSelfSignedCertKeySize)),
			expectedValue: "4096",
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
			constName:     "WatcherImageName",
			constValue:    DefaultWatcherImageName,
			expectedValue: "runtime-watcher",
		},
		{
			constName:     "WatcherImageRegistry",
			constValue:    DefaultWatcherImageRegistry,
			expectedValue: "europe-docker.pkg.dev/kyma-project/prod",
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
			constName:     "DefaultDropCrdStoredVersionMap",
			constValue:    DefaultDropCrdStoredVersionMap,
			expectedValue: "Manifest:v1beta1,Watcher:v1beta1,ModuleTemplate:v1beta1,Kyma:v1beta1",
		},
		{
			constName:     "DefaultLeaderElectionLeaseDuration",
			constValue:    DefaultLeaderElectionLeaseDuration.String(),
			expectedValue: (180 * time.Second).String(),
		},
		{
			constName:     "DefaultLeaderElectionRenewDeadline",
			constValue:    DefaultLeaderElectionRenewDeadline.String(),
			expectedValue: (120 * time.Second).String(),
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

func Test_Flags_Validate(t *testing.T) {
	tests := []struct {
		name  string
		flags FlagVar
		err   error
	}{
		{
			name:  "WatcherImageTag is required",
			flags: newFlagVarBuilder().withEnabledKcpWatcher(true).withWatcherImageTag("").build(),
			err:   ErrMissingWatcherImageTag,
		},
		{
			name:  "WatcherImageTag is NOT required",
			flags: newFlagVarBuilder().withWatcherImageTag("").build(),
			err:   nil,
		},
		{
			name:  "WatcherImageRegistry is required",
			flags: newFlagVarBuilder().withEnabledKcpWatcher(true).withWatcherImageRegistry("").build(),
			err:   ErrMissingWatcherImageRegistry,
		},
		{
			name:  "WatcherImageRegistry is NOT required",
			flags: newFlagVarBuilder().withWatcherImageRegistry("").build(),
			err:   nil,
		},
		{
			name:  "WatcherResourcesPath is required",
			flags: newFlagVarBuilder().withEnabledKcpWatcher(true).withWatcherResourcesPath("").build(),
			err:   ErrWatcherDirNotExist,
		},
		{
			name:  "WatcherResourcesPath is NOT required",
			flags: newFlagVarBuilder().withWatcherResourcesPath("").build(),
			err:   nil,
		},
		{
			name:  "LeaderElectionRenewDeadline > LeaderElectionLeaseDuration",
			flags: newFlagVarBuilder().withLeaderElectionRenewDeadline(2).withLeaderElectionLeaseDuration(1).build(),
			err:   ErrLeaderElectionTimeoutConfig,
		},
		{
			name:  "LeaderElectionRenewDeadline = LeaderElectionLeaseDuration",
			flags: newFlagVarBuilder().withLeaderElectionRenewDeadline(1).withLeaderElectionLeaseDuration(1).build(),
			err:   ErrLeaderElectionTimeoutConfig,
		},
		{
			name:  "LeaderElectionRenewDeadline < LeaderElectionLeaseDuration",
			flags: newFlagVarBuilder().withLeaderElectionRenewDeadline(1).withLeaderElectionLeaseDuration(2).build(),
			err:   nil,
		},
		{
			name:  "SelfSignedCertKeySize 2048",
			flags: newFlagVarBuilder().withSelfSignedCertKeySize(2048).build(),
			err:   ErrInvalidSelfSignedCertKeyLength,
		},
		{
			name:  "SelfSignedCertKeySize 8192",
			flags: newFlagVarBuilder().withSelfSignedCertKeySize(8192).build(),
			err:   ErrInvalidSelfSignedCertKeyLength,
		},
		{
			name:  "SelfSignedCertKeySize 4711",
			flags: newFlagVarBuilder().withSelfSignedCertKeySize(4711).build(),
			err:   ErrInvalidSelfSignedCertKeyLength,
		},
		{
			name:  "SelfSignedCertKeySize 4096",
			flags: newFlagVarBuilder().withSelfSignedCertKeySize(4096).build(),
			err:   nil,
		},
		{
			name:  "ManifestRequeueJitterProbability < 0",
			flags: newFlagVarBuilder().withManifestRequeueJitterProbability(-1).build(),
			err:   ErrInvalidManifestRequeueJitterPercentage,
		},
		{
			name:  "ManifestRequeueJitterProbability > 0.05",
			flags: newFlagVarBuilder().withManifestRequeueJitterProbability(0.1).build(),
			err:   ErrInvalidManifestRequeueJitterPercentage,
		},
		{
			name:  "ManifestRequeueJitterProbability 0.01",
			flags: newFlagVarBuilder().withManifestRequeueJitterProbability(0.01).build(),
			err:   nil,
		},
		{
			name:  "ManifestRequeueJitterPercentage < 0",
			flags: newFlagVarBuilder().withManifestRequeueJitterPercentage(-1).build(),
			err:   ErrInvalidManifestRequeueJitterProbability,
		},
		{
			name:  "ManifestRequeueJitterPercentage > 0.05",
			flags: newFlagVarBuilder().withManifestRequeueJitterPercentage(2).build(),
			err:   ErrInvalidManifestRequeueJitterProbability,
		},
		{
			name:  "ManifestRequeueJitterPercentage 0.1",
			flags: newFlagVarBuilder().withManifestRequeueJitterPercentage(0.1).build(),
			err:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.flags.Validate()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// test builder

type flagVarBuilder struct {
	flags FlagVar
}

func newFlagVarBuilder() *flagVarBuilder {
	builder := &flagVarBuilder{
		flags: FlagVar{},
	}

	return builder.
		withEnabledKcpWatcher(false).
		withWatcherImageTag("v1.0.0").
		withWatcherImageName("runtime-watcher").
		withWatcherImageRegistry("foo.bar").
		withWatcherResourcesPath("./skr-webhook").
		withLeaderElectionRenewDeadline(120 * time.Second).
		withLeaderElectionLeaseDuration(180 * time.Second).
		withSelfSignedCertKeySize(4096).
		withManifestRequeueJitterProbability(0.01).
		withManifestRequeueJitterPercentage(0.1)
}

func (b *flagVarBuilder) build() FlagVar {
	return b.flags
}

func (b *flagVarBuilder) withEnabledKcpWatcher(enabled bool) *flagVarBuilder {
	b.flags.EnableKcpWatcher = enabled
	return b
}

func (b *flagVarBuilder) withWatcherImageTag(tag string) *flagVarBuilder {
	b.flags.WatcherImageTag = tag
	return b
}

func (b *flagVarBuilder) withWatcherImageName(name string) *flagVarBuilder {
	b.flags.WatcherImageName = name
	return b
}

func (b *flagVarBuilder) withWatcherImageRegistry(registry string) *flagVarBuilder {
	b.flags.WatcherImageRegistry = registry
	return b
}

func (b *flagVarBuilder) withWatcherResourcesPath(path string) *flagVarBuilder {
	b.flags.WatcherResourcesPath = path
	return b
}

func (b *flagVarBuilder) withLeaderElectionRenewDeadline(duration time.Duration) *flagVarBuilder {
	b.flags.LeaderElectionRenewDeadline = duration
	return b
}

func (b *flagVarBuilder) withLeaderElectionLeaseDuration(duration time.Duration) *flagVarBuilder {
	b.flags.LeaderElectionLeaseDuration = duration
	return b
}

func (b *flagVarBuilder) withSelfSignedCertKeySize(size int) *flagVarBuilder {
	b.flags.SelfSignedCertKeySize = size
	return b
}

func (b *flagVarBuilder) withManifestRequeueJitterProbability(probability float64) *flagVarBuilder {
	b.flags.ManifestRequeueJitterProbability = probability
	return b
}

func (b *flagVarBuilder) withManifestRequeueJitterPercentage(percentage float64) *flagVarBuilder {
	b.flags.ManifestRequeueJitterPercentage = percentage
	return b
}
