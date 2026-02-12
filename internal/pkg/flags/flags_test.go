package flags_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/common"
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
			constName:     "DefaultCertificateManagement",
			constValue:    certmanagerv1.SchemeGroupVersion.String(),
			expectedValue: "cert-manager.io/v1",
		},
		{
			constName:     "DefaultKymaRequeueSuccessInterval",
			constValue:    DefaultKymaRequeueSuccessInterval.String(),
			expectedValue: (5 * time.Minute).String(),
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
			expectedValue: (5 * time.Minute).String(),
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
			expectedValue: (1 * time.Minute).String(),
		},
		{
			constName:     "DefaultClientQPS",
			constValue:    strconv.Itoa(DefaultClientQPS),
			expectedValue: "1000",
		},
		{
			constName:     "DefaultClientBurst",
			constValue:    strconv.Itoa(DefaultClientBurst),
			expectedValue: "2000",
		},
		{
			constName:     "DefaultSkrClientQPS",
			constValue:    strconv.Itoa(DefaultSkrClientQPS),
			expectedValue: "50",
		},
		{
			constName:     "DefaultSkrClientBurst",
			constValue:    strconv.Itoa(DefaultSkrClientBurst),
			expectedValue: "100",
		},
		{
			constName:     "DefaultPprofServerTimeout",
			constValue:    DefaultPprofServerTimeout.String(),
			expectedValue: (90 * time.Second).String(),
		},
		{
			constName:     "RateLimiterBurstDefault",
			constValue:    strconv.Itoa(RateLimiterBurstDefault),
			expectedValue: "2000",
		},
		{
			constName:     "RateLimiterFrequencyDefault",
			constValue:    strconv.Itoa(RateLimiterFrequencyDefault),
			expectedValue: "1000",
		},
		{
			constName:     "FailureBaseDelayDefault",
			constValue:    FailureBaseDelayDefault.String(),
			expectedValue: (5 * time.Second).String(),
		},
		{
			constName:     "FailureMaxDelayDefault",
			constValue:    FailureMaxDelayDefault.String(),
			expectedValue: (30 * time.Second).String(),
		},
		{
			constName:     "DefaultCacheSyncTimeout",
			constValue:    DefaultCacheSyncTimeout.String(),
			expectedValue: (60 * time.Minute).String(),
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
			expectedValue: "klm-watcher",
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
			constName:     "DefaultSelfSignedCertIssuerNamespace",
			constValue:    DefaultSelfSignedCertIssuerNamespace,
			expectedValue: "istio-system",
		},
		{
			constName:     "DefaultSelfSignedCertDuration",
			constValue:    DefaultSelfSignedCertDuration.String(),
			expectedValue: (60 * 24 * time.Hour).String(),
		},
		{
			constName:     "DefaultSelfSignedCertRenewBefore",
			constValue:    DefaultSelfSignedCertRenewBefore.String(),
			expectedValue: (30 * 24 * time.Hour).String(),
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
			constName:     "DefaultSelfSignedCertificateIssuerName",
			constValue:    DefaultSelfSignedCertificateIssuerName,
			expectedValue: "klm-watcher-selfsigned",
		},
		{
			constName:     "DefaultIstioGatewayServerCertSwitchGracePeriod",
			constValue:    DefaultIstioGatewayServerCertSwitchGracePeriod.String(),
			expectedValue: (4 * 24 * time.Hour).String(),
		},
		{
			constName:     "DefaultIstioGatewaySecretRequeueSuccessInterval",
			constValue:    DefaultIstioGatewaySecretRequeueSuccessInterval.String(),
			expectedValue: (5 * time.Minute).String(),
		},
		{
			constName:     "DefaultIstioGatewaySecretRequeueErrInterval",
			constValue:    DefaultIstioGatewaySecretRequeueErrInterval.String(),
			expectedValue: (2 * time.Second).String(),
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
			constName:     "DefaultMinMaintenanceWindowSize",
			constValue:    DefaultMinMaintenanceWindowSize.String(),
			expectedValue: (20 * time.Minute).String(),
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
		{
			constName:     "DefaultLeaderElectionRetryPeriod",
			constValue:    DefaultLeaderElectionRetryPeriod.String(),
			expectedValue: (3 * time.Second).String(),
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
			name:  "CertificateManagement cert-manager.io/v1",
			flags: newFlagVarBuilder().withCertificateManagement(certmanagerv1.SchemeGroupVersion.String()).build(),
			err:   nil,
		},
		{
			name:  "CertificateManagement cert.gardener.cloud/v1alpha1",
			flags: newFlagVarBuilder().withCertificateManagement(gcertv1alpha1.SchemeGroupVersion.String()).build(),
			err:   nil,
		},
		{
			name:  "CertificateManagement unsupported",
			flags: newFlagVarBuilder().withCertificateManagement("foobar").build(),
			err:   common.ErrUnsupportedCertificateManagementSystem,
		},
		{
			name:  "WatcherImageTag is required",
			flags: newFlagVarBuilder().withWatcherImageTag("").build(),
			err:   ErrMissingWatcherImageTag,
		},
		{
			name:  "WatcherImageRegistry is required",
			flags: newFlagVarBuilder().withWatcherImageRegistry("").build(),
			err:   ErrMissingWatcherImageRegistry,
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
		{
			name:  "Neither OciRegistryHost nor OciRegistryCredSecret provided",
			flags: newFlagVarBuilder().withOciRegistryHost("").withOciRegistryCredSecretName("").build(),
			err:   common.ErrNoOCIRegistryHostAndCredSecret,
		},
		{
			name:  "Both OciRegistryHost and OciRegistryCredSecret provided",
			flags: newFlagVarBuilder().withOciRegistryHost("test").withOciRegistryCredSecretName("test").build(),
			err:   common.ErrBothOCIRegistryHostAndCredSecretProvided,
		},
		{
			name: "SelfSignedCertRenewBuffer >= SwitchGracePeriod",
			flags: newFlagVarBuilder().withSelfSignedCertRenewBuffer(48 * time.Hour).
				withIstioGatewayServerCertSwitchGracePeriod(24 * time.Hour).build(),
			err: ErrSelfSignedCertRenewBufferExceedsGracePeriod,
		},
		{
			name: "SelfSignedCertRenewBuffer == SwitchGracePeriod",
			flags: newFlagVarBuilder().withSelfSignedCertRenewBuffer(24 * time.Hour).
				withIstioGatewayServerCertSwitchGracePeriod(24 * time.Hour).build(),
			err: ErrSelfSignedCertRenewBufferExceedsGracePeriod,
		},
		{
			name: "SelfSignedCertRenewBuffer < SwitchGracePeriod",
			flags: newFlagVarBuilder().withSelfSignedCertRenewBuffer(12 * time.Hour).
				withIstioGatewayServerCertSwitchGracePeriod(24 * time.Hour).build(),
			err: nil,
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
		withCertificateManagement(certmanagerv1.SchemeGroupVersion.String()).
		withWatcherImageTag("v1.0.0").
		withWatcherImageName("runtime-watcher").
		withWatcherImageRegistry("foo.bar").
		withLeaderElectionRenewDeadline(120 * time.Second).
		withLeaderElectionLeaseDuration(180 * time.Second).
		withSelfSignedCertKeySize(4096).
		withManifestRequeueJitterProbability(0.01).
		withManifestRequeueJitterPercentage(0.1).
		withOciRegistryHost("europe-docker.pkg.dev").
		withSelfSignedCertRenewBuffer(DefaultSelfSignedCertificateRenewBuffer).
		withIstioGatewayServerCertSwitchGracePeriod(DefaultIstioGatewayServerCertSwitchGracePeriod)
}

func (b *flagVarBuilder) build() FlagVar {
	return b.flags
}

func (b *flagVarBuilder) withCertificateManagement(certificateManagement string) *flagVarBuilder {
	b.flags.CertificateManagement = certificateManagement
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

func (b *flagVarBuilder) withOciRegistryHost(host string) *flagVarBuilder {
	b.flags.OciRegistryHost = host
	return b
}

func (b *flagVarBuilder) withOciRegistryCredSecretName(secretName string) *flagVarBuilder {
	b.flags.OciRegistryCredSecretName = secretName
	return b
}

func (b *flagVarBuilder) withSelfSignedCertRenewBuffer(d time.Duration) *flagVarBuilder {
	b.flags.SelfSignedCertRenewBuffer = d
	return b
}

func (b *flagVarBuilder) withIstioGatewayServerCertSwitchGracePeriod(d time.Duration) *flagVarBuilder {
	b.flags.IstioGatewayServerCertSwitchGracePeriod = d
	return b
}
