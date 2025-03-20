package flags

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	DefaultKymaRequeueSuccessInterval                                   = 30 * time.Second
	DefaultKymaRequeueErrInterval                                       = 2 * time.Second
	DefaultKymaRequeueWarningInterval                                   = 30 * time.Second
	DefaultKymaRequeueBusyInterval                                      = 5 * time.Second
	DefaultManifestRequeueSuccessInterval                               = 30 * time.Second
	DefaultManifestRequeueErrInterval                                   = 2 * time.Second
	DefaultManifestRequeueWarningInterval                               = 30 * time.Second
	DefaultManifestRequeueBusyInterval                                  = 5 * time.Second
	DefaultManifestRequeueJitterProbability                             = 0.02
	DefaultManifestRequeueJitterPercentage                              = 0.02
	DefaultMandatoryModuleRequeueSuccessInterval                        = 30 * time.Second
	DefaultMandatoryModuleDeletionRequeueSuccessInterval                = 30 * time.Second
	DefaultWatcherRequeueSuccessInterval                                = 30 * time.Second
	DefaultClientQPS                                                    = 300
	DefaultClientBurst                                                  = 600
	DefaultPprofServerTimeout                                           = 90 * time.Second
	RateLimiterBurstDefault                                             = 200
	RateLimiterFrequencyDefault                                         = 30
	FailureBaseDelayDefault                                             = 100 * time.Millisecond
	FailureMaxDelayDefault                                              = 5 * time.Second
	DefaultCacheSyncTimeout                                             = 2 * time.Minute
	DefaultLogLevel                                                     = log.WarnLevel
	DefaultPurgeFinalizerTimeout                                        = 5 * time.Minute
	DefaultMaxConcurrentManifestReconciles                              = 1
	DefaultMaxConcurrentKymaReconciles                                  = 1
	DefaultMaxConcurrentWatcherReconciles                               = 1
	DefaultMaxConcurrentMandatoryModuleReconciles                       = 1
	DefaultMaxConcurrentMandatoryModuleDeletionReconciles               = 1
	DefaultIstioGatewayName                                             = "klm-watcher"
	DefaultIstioGatewayNamespace                                        = "kcp-system"
	DefaultIstioNamespace                                               = "istio-system"
	DefaultCaCertName                                                   = "klm-watcher-serving"
	DefaultSelfSignedCertDuration                         time.Duration = 1441 * time.Hour // must be smaller than duration of root cert, greater than 1440
	DefaultSelfSignedCertRenewBefore                      time.Duration = 60 * 24 * time.Hour
	DefaultSelfSignedCertificateRenewBuffer                             = 24 * time.Hour
	DefaultSelfSignedCertKeySize                                        = 2048 // Gardener cert-manager seems to have problems with RSA 4096 keys
	DefaultIstioGatewayCertSwitchBeforeExpirationTime                   = 24 * time.Hour
	DefaultIstioGatewaySecretRequeueSuccessInterval                     = 5 * time.Minute
	DefaultIstioGatewaySecretRequeueErrInterval                         = 2 * time.Second
	DefaultRemoteSyncNamespace                                          = shared.DefaultRemoteNamespace
	DefaultMetricsAddress                                               = ":8080"
	DefaultProbeAddress                                                 = ":8081"
	DefaultKymaListenerAddress                                          = ":8082"
	DefaultManifestListenerAddress                                      = ":8083"
	DefaultPprofAddress                                                 = ":8084"
	DefaultWatcherImageName                                             = "runtime-watcher"
	DefaultWatcherImageRegistry                                         = "europe-docker.pkg.dev/kyma-project/prod"
	DefaultWatcherResourcesPath                                         = "./skr-webhook"
	DefaultWatcherResourceLimitsCPU                                     = "0.1"
	DefaultWatcherResourceLimitsMemory                                  = "200Mi"
	DefaultDropCrdStoredVersionMap                                      = "Manifest:v1beta1,Watcher:v1beta1,ModuleTemplate:v1beta1,Kyma:v1beta1"
	DefaultMetricsCleanupIntervalInMinutes                              = 15
	DefaultMinMaintenanceWindowSize                                     = 20 * time.Minute
	DefaultLeaderElectionLeaseDuration                                  = 180 * time.Second
	DefaultLeaderElectionRenewDeadline                                  = 120 * time.Second
	DefaultLeaderElectionRetryPeriod                                    = 3 * time.Second
)

var (
	ErrMissingWatcherImageTag                  = errors.New("runtime watcher image tag is not provided")
	ErrMissingWatcherImageRegistry             = errors.New("runtime watcher image registry is not provided")
	ErrWatcherDirNotExist                      = errors.New("failed to locate watcher resource manifest folder")
	ErrLeaderElectionTimeoutConfig             = errors.New("configured leader-election-renew-deadline must be less than leader-election-lease-duration")
	ErrInvalidSelfSignedCertKeyLength          = errors.New("invalid self-signed-cert-key-size: must be 4096")
	ErrInvalidManifestRequeueJitterPercentage  = errors.New("invalid manifest requeue jitter percentage: must be between 0 and 0.05")
	ErrInvalidManifestRequeueJitterProbability = errors.New("invalid manifest requeue jitter probability: must be between 0 and 1")
)

//nolint:funlen // defines all program flags
func DefineFlagVar() *FlagVar {
	flagVar := new(FlagVar)
	flag.StringVar(&flagVar.MetricsAddr, "metrics-bind-address", DefaultMetricsAddress,
		"The address the metric endpoint binds to.")
	flag.StringVar(&flagVar.ProbeAddr, "health-probe-bind-address", DefaultProbeAddress,
		"The address the probe endpoint binds to.")
	flag.StringVar(&flagVar.KymaListenerAddr, "kyma-skr-listener-bind-address", DefaultKymaListenerAddress,
		"The address the skr listener endpoint binds to.")
	flag.StringVar(&flagVar.ManifestListenerAddr, "manifest-skr-listener-bind-address", DefaultManifestListenerAddress,
		"The address the skr listener endpoint binds to.")
	flag.StringVar(&flagVar.PprofAddr, "pprof-bind-address", DefaultPprofAddress,
		"The address the pprof endpoint binds to.")
	flag.IntVar(&flagVar.MaxConcurrentKymaReconciles, "max-concurrent-kyma-reconciles",
		DefaultMaxConcurrentKymaReconciles, "The maximum number of concurrent Kyma Reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentManifestReconciles, "max-concurrent-manifest-reconciles",
		DefaultMaxConcurrentManifestReconciles,
		"The maximum number of concurrent Manifest Reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentWatcherReconciles, "max-concurrent-watcher-reconciles",
		DefaultMaxConcurrentWatcherReconciles,
		"The maximum number of concurrent Watcher Reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentMandatoryModuleReconciles, "max-concurrent-mandatory-modules-reconciles",
		DefaultMaxConcurrentMandatoryModuleReconciles,
		"The maximum number of concurrent Mandatory Modules Reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentMandatoryModuleDeletionReconciles,
		"max-concurrent-mandatory-modules-deletion-reconciles",
		DefaultMaxConcurrentMandatoryModuleDeletionReconciles,
		"The maximum number of concurrent Mandatory Modules Deletion Reconciles which can be run.")
	flag.BoolVar(&flagVar.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&flagVar.LeaderElectionLeaseDuration, "leader-election-lease-duration",
		DefaultLeaderElectionLeaseDuration,
		"Configures the 'LeaseDuration' option of the controller-runtime library used to run the controller manager process.")
	flag.DurationVar(&flagVar.LeaderElectionRenewDeadline, "leader-election-renew-deadline",
		DefaultLeaderElectionRenewDeadline,
		"Configures the 'RenewDeadline' option of the controller-runtime library used to run the controller manager process.")
	flag.DurationVar(&flagVar.LeaderElectionRetryPeriod, "leader-election-retry-period",
		DefaultLeaderElectionRetryPeriod,
		"Configures the 'RetryPeriod' option of the controller-runtime library used to run the controller manager process.")
	flag.DurationVar(&flagVar.KymaRequeueSuccessInterval, "kyma-requeue-success-interval",
		DefaultKymaRequeueSuccessInterval,
		"determines the duration a Kyma in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.KymaRequeueErrInterval, "kyma-requeue-error-interval",
		DefaultKymaRequeueErrInterval,
		"determines the duration after which a Kyma in Error state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.KymaRequeueWarningInterval, "kyma-requeue-warning-interval",
		DefaultKymaRequeueWarningInterval,
		"determines the duration after which a Kyma in Warning state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.KymaRequeueBusyInterval, "kyma-requeue-busy-interval",
		DefaultKymaRequeueBusyInterval,
		"determines the duration after which a Kyma in Processing state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.MandatoryModuleRequeueSuccessInterval, "mandatory-module-requeue-success-interval",
		DefaultMandatoryModuleRequeueSuccessInterval,
		"determines the duration a Kyma in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueSuccessInterval, "manifest-requeue-success-interval",
		DefaultManifestRequeueSuccessInterval,
		"determines the duration a Manifest in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueErrInterval, "manifest-requeue-error-interval",
		DefaultManifestRequeueErrInterval,
		"determines the duration a Manifest in Error state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueWarningInterval, "manifest-requeue-warning-interval",
		DefaultManifestRequeueWarningInterval,
		"determines the duration a Manifest in Warning state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueBusyInterval, "manifest-requeue-busy-interval",
		DefaultManifestRequeueBusyInterval,
		"determines the duration a Manifest in Processing state is enqueued for reconciliation.")
	flag.Float64Var(&flagVar.ManifestRequeueJitterProbability, "manifest-requeue-jitter-probability",
		DefaultManifestRequeueJitterProbability,
		"determines the probability that jitter is applied to the requeue interval.")
	flag.Float64Var(&flagVar.ManifestRequeueJitterPercentage, "manifest-requeue-jitter-percentage",
		DefaultManifestRequeueJitterPercentage,
		"determines the percentage range for the requeue jitter applied to the requeue interval. "+
			"E.g., 0.1 means +/- 10% of the interval.")
	flag.DurationVar(&flagVar.MandatoryModuleDeletionRequeueSuccessInterval,
		"mandatory-module-deletion-requeue-success-interval",
		DefaultMandatoryModuleDeletionRequeueSuccessInterval,
		"determines the duration a Kyma in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.WatcherRequeueSuccessInterval, "watcher-requeue-success-interval",
		DefaultWatcherRequeueSuccessInterval,
		"determines the duration a Watcher in Ready state is enqueued for reconciliation.")

	flag.Float64Var(&flagVar.ClientQPS, "k8s-client-qps", DefaultClientQPS, "kubernetes client QPS")
	flag.IntVar(&flagVar.ClientBurst, "k8s-client-burst", DefaultClientBurst, "kubernetes client Burst")
	flag.BoolVar(&flagVar.EnableWebhooks, "enable-webhooks", false,
		"Enabling Validation/Conversion Webhooks.")
	flag.BoolVar(&flagVar.EnableKcpWatcher, "enable-kcp-watcher", false,
		"Enabling KCP Watcher to reconcile Watcher CRs created by KCP run operators")
	flag.StringVar(&flagVar.AdditionalDNSNames, "additional-dns-names", "",
		"Additional DNS Names which are added to Kyma Certificates as SANs. Input should be given as "+
			"comma-separated list, for example \"--additional-dns-names=localhost,127.0.0.1,host.k3d.internal\".")
	flag.StringVar(&flagVar.IstioNamespace, "istio-namespace", DefaultIstioNamespace,
		"Cluster Resource Namespace of Istio")
	flag.StringVar(&flagVar.IstioGatewayName, "istio-gateway-name", DefaultIstioGatewayName,
		"Cluster Resource Name of Istio Gateway")
	flag.StringVar(&flagVar.IstioGatewayNamespace, "istio-gateway-namespace", DefaultIstioGatewayNamespace,
		"Cluster Resource Namespace of Istio Gateway")
	flag.StringVar(&flagVar.ListenerPortOverwrite, "listener-port-overwrite", "",
		"Port that is mapped to HTTP port of the local k3d cluster using --port 9443:443@loadbalancer when "+
			"creating the KCP cluster")
	flag.BoolVar(&flagVar.Pprof, "pprof", false, "Whether to start up a pprof server.")
	flag.DurationVar(&flagVar.PprofServerTimeout, "pprof-server-timeout", DefaultPprofServerTimeout,
		"Timeout of Read / Write for the pprof server.")
	flag.IntVar(&flagVar.RateLimiterBurst, "rate-limiter-burst", RateLimiterBurstDefault,
		"Indicates the rateLimiterBurstDefault value for the bucket rate limiter.")
	flag.IntVar(&flagVar.RateLimiterFrequency, "rate-limiter-frequency", RateLimiterFrequencyDefault,
		"Indicates the bucket rate limiter frequency, signifying no. of events per second.")
	flag.DurationVar(&flagVar.FailureBaseDelay, "failure-base-delay", FailureBaseDelayDefault,
		"Indicates the failure base delay in seconds for rate limiter.")
	flag.DurationVar(&flagVar.FailureMaxDelay, "failure-max-delay", FailureMaxDelayDefault,
		"Indicates the failure max delay in seconds")
	flag.DurationVar(&flagVar.CacheSyncTimeout, "cache-sync-timeout", DefaultCacheSyncTimeout,
		"Indicates the cache sync timeout in seconds")
	flag.BoolVar(&flagVar.EnableDomainNameVerification, "enable-domain-name-pinning", true,
		"Enabling verification of incoming listener request by comparing SAN with KymaCR-SKR-domain")
	flag.IntVar(
		&flagVar.LogLevel, "log-level", DefaultLogLevel,
		"indicates the current log-level, enter negative values to increase verbosity (e.g. 9)",
	)
	flag.BoolVar(&flagVar.InKCPMode, "in-kcp-mode", false,
		"Indicates lifecycle manager is deployed in control-plane mode (multiple clusters mode)")
	flag.BoolVar(&flagVar.EnablePurgeFinalizer, "enable-purge-finalizer", false,
		"Enabling purge finalizer")
	flag.DurationVar(&flagVar.PurgeFinalizerTimeout, "purge-finalizer-timeout", DefaultPurgeFinalizerTimeout,
		"Indicates the SKR Purge Finalizers execution delay in seconds")
	flag.StringVar(&flagVar.SkipPurgingFor, "skip-finalizer-purging-for", "", "Exclude the passed CRDs"+
		" from finalizer removal. Example: 'ingressroutetcps.traefik.containo.us,*.helm.cattle.io'.")
	flag.StringVar(&flagVar.RemoteSyncNamespace, "sync-namespace", DefaultRemoteSyncNamespace,
		"Name of the namespace for syncing remote Kyma and module catalog")
	flag.StringVar(&flagVar.CaCertName, "ca-cert-name", DefaultCaCertName,
		"Name of the CA Certificate in Istio Namespace which is used to sign SKR Certificates")
	flag.DurationVar(&flagVar.SelfSignedCertDuration, "self-signed-cert-duration", DefaultSelfSignedCertDuration,
		"The lifetime duration of self-signed certificate, minimum accepted duration is 1 hour.")
	flag.DurationVar(&flagVar.SelfSignedCertRenewBefore, "self-signed-cert-renew-before",
		DefaultSelfSignedCertRenewBefore,
		"How long before the currently issued self-signed certificate's expiry cert-manager should renew the certificate")
	flag.DurationVar(&flagVar.SelfSignedCertRenewBuffer, "self-signed-cert-renew-buffer",
		DefaultSelfSignedCertificateRenewBuffer,
		"The buffer duration to wait before confirm self-signed certificate not renewed")
	flag.IntVar(&flagVar.SelfSignedCertKeySize, "self-signed-cert-key-size", DefaultSelfSignedCertKeySize,
		"The key size for the self-signed certificate")
	flag.DurationVar(&flagVar.IstioGatewayCertSwitchBeforeExpirationTime,
		"istio-gateway-cert-switch-before-expiration-time", DefaultIstioGatewayCertSwitchBeforeExpirationTime,
		"Time before the expiration of the current CA certificate when the Gateway certificate should be switched")
	flag.StringVar(&flagVar.SelfSignedCertIssuerNamespace, "self-signed-cert-issuer-namespace", "istio-system", "Namespace of the issuer for self-signed certificates")
	flag.DurationVar(&flagVar.IstioGatewaySecretRequeueSuccessInterval,
		"istio-gateway-secret-requeue-success-interval", DefaultIstioGatewaySecretRequeueSuccessInterval,
		"determines the duration after which the istio gateway secret is enqueued after successful reconciliation.")
	flag.DurationVar(&flagVar.IstioGatewaySecretRequeueErrInterval,
		"istio-gateway-secret-requeue-error-interval", DefaultIstioGatewaySecretRequeueErrInterval,
		"determines the duration after which the istio gateway secret is enqueued after unsuccessful reconciliation.")
	flag.BoolVar(&flagVar.UseLegacyStrategyForIstioGatewaySecret, "legacy-strategy-for-istio-gateway-secret",
		false, "Use the legacy strategy (with downtime) for the Istio Gateway Secret")
	flag.BoolVar(&flagVar.IsKymaManaged, "is-kyma-managed", false, "indicates whether Kyma is managed")
	flag.StringVar(&flagVar.DropCrdStoredVersionMap, "drop-crd-stored-version-map", DefaultDropCrdStoredVersionMap,
		"Specify the API versions to be dropped from the storage version. The input format should be a "+
			"comma-separated list of API versions, where each API version is in the format 'kind:version'.")
	flag.StringVar(&flagVar.WatcherImageName, "skr-watcher-image-name", DefaultWatcherImageName,
		`Image name to be used for the SKR watcher image.`)
	flag.StringVar(&flagVar.WatcherImageTag, "skr-watcher-image-tag", "",
		`Image tag to be used for the SKR watcher image.`)
	flag.StringVar(&flagVar.WatcherImageRegistry, "skr-watcher-image-registry", DefaultWatcherImageRegistry,
		`Image registry to be used for the SKR watcher image.`)
	flag.StringVar(&flagVar.WatcherResourceLimitsMemory, "skr-webhook-memory-limits",
		DefaultWatcherResourceLimitsMemory,
		"The resources.limits.memory for skr webhook.")
	flag.StringVar(&flagVar.WatcherResourceLimitsCPU, "skr-webhook-cpu-limits", DefaultWatcherResourceLimitsCPU,
		"The resources.limits.cpu for skr webhook.")
	flag.StringVar(&flagVar.WatcherResourcesPath, "skr-watcher-path", DefaultWatcherResourcesPath,
		"The path to the skr watcher resources.")
	flag.IntVar(&flagVar.MetricsCleanupIntervalInMinutes, "metrics-cleanup-interval",
		DefaultMetricsCleanupIntervalInMinutes,
		"The interval at which the cleanup of non-existing kyma CRs metrics runs.")
	flag.DurationVar(&flagVar.MinMaintenanceWindowSize, "min-maintenance-window-size",
		DefaultMinMaintenanceWindowSize,
		"The minimum duration of maintenance window required for reconciling modules with downtime.")
	return flagVar
}

type FlagVar struct {
	MetricsAddr                                    string
	EnableDomainNameVerification                   bool
	EnableLeaderElection                           bool
	LeaderElectionLeaseDuration                    time.Duration
	LeaderElectionRenewDeadline                    time.Duration
	LeaderElectionRetryPeriod                      time.Duration
	EnablePurgeFinalizer                           bool
	EnableKcpWatcher                               bool
	EnableWebhooks                                 bool
	ProbeAddr                                      string
	KymaListenerAddr, ManifestListenerAddr         string
	MaxConcurrentKymaReconciles                    int
	MaxConcurrentManifestReconciles                int
	MaxConcurrentWatcherReconciles                 int
	MaxConcurrentMandatoryModuleReconciles         int
	MaxConcurrentMandatoryModuleDeletionReconciles int
	KymaRequeueSuccessInterval                     time.Duration
	KymaRequeueErrInterval                         time.Duration
	KymaRequeueBusyInterval                        time.Duration
	KymaRequeueWarningInterval                     time.Duration
	ManifestRequeueSuccessInterval                 time.Duration
	ManifestRequeueErrInterval                     time.Duration
	ManifestRequeueBusyInterval                    time.Duration
	ManifestRequeueWarningInterval                 time.Duration
	WatcherRequeueSuccessInterval                  time.Duration
	MandatoryModuleRequeueSuccessInterval          time.Duration
	MandatoryModuleDeletionRequeueSuccessInterval  time.Duration
	ClientQPS                                      float64
	ClientBurst                                    int
	IstioNamespace                                 string
	IstioGatewayName                               string
	IstioGatewayNamespace                          string
	AdditionalDNSNames                             string
	// ListenerPortOverwrite is used to enable the user to overwrite the port
	// used to expose the KCP cluster for the watcher. By default, it will be
	// fetched from the specified gateway.
	ListenerPortOverwrite                      string
	Pprof                                      bool
	PprofAddr                                  string
	PprofServerTimeout                         time.Duration
	FailureBaseDelay, FailureMaxDelay          time.Duration
	RateLimiterBurst, RateLimiterFrequency     int
	CacheSyncTimeout                           time.Duration
	LogLevel                                   int
	InKCPMode                                  bool
	PurgeFinalizerTimeout                      time.Duration
	SkipPurgingFor                             string
	RemoteSyncNamespace                        string
	CaCertName                                 string
	IsKymaManaged                              bool
	SelfSignedCertDuration                     time.Duration
	SelfSignedCertRenewBefore                  time.Duration
	SelfSignedCertRenewBuffer                  time.Duration
	SelfSignedCertKeySize                      int
	SelfSignedCertIssuerNamespace              string
	UseLegacyStrategyForIstioGatewaySecret     bool
	DropCrdStoredVersionMap                    string
	WatcherImageTag                            string
	WatcherImageName                           string
	WatcherImageRegistry                       string
	WatcherResourceLimitsMemory                string
	WatcherResourceLimitsCPU                   string
	WatcherResourcesPath                       string
	MetricsCleanupIntervalInMinutes            int
	ManifestRequeueJitterProbability           float64
	ManifestRequeueJitterPercentage            float64
	IstioGatewayCertSwitchBeforeExpirationTime time.Duration
	IstioGatewaySecretRequeueSuccessInterval   time.Duration
	IstioGatewaySecretRequeueErrInterval       time.Duration
	MinMaintenanceWindowSize                   time.Duration
}

func (f FlagVar) Validate() error {
	if f.EnableKcpWatcher {
		if f.WatcherImageTag == "" {
			return ErrMissingWatcherImageTag
		}
		if f.WatcherImageRegistry == "" {
			return ErrMissingWatcherImageRegistry
		}
		dirInfo, err := os.Stat(f.WatcherResourcesPath)
		if err != nil || !dirInfo.IsDir() {
			return ErrWatcherDirNotExist
		}
	}

	if f.LeaderElectionRenewDeadline >= f.LeaderElectionLeaseDuration {
		return fmt.Errorf("%w (%.1f[s])", ErrLeaderElectionTimeoutConfig, f.LeaderElectionLeaseDuration.Seconds())
	}

	if !map[int]bool{
		// Gardener cert-manager seems to have problems with RSA 4096 keys
		2048: true, // 2048 is a valid value for cert-manager, but explicitly prohibited as not compliant to security requirements
		4096: true,
		8192: false, // see https://github.com/kyma-project/lifecycle-manager/issues/1793
	}[f.SelfSignedCertKeySize] {
		return ErrInvalidSelfSignedCertKeyLength
	}

	if f.ManifestRequeueJitterProbability < 0 || f.ManifestRequeueJitterProbability > 0.05 {
		return ErrInvalidManifestRequeueJitterPercentage
	}
	if f.ManifestRequeueJitterPercentage < 0 || f.ManifestRequeueJitterPercentage > 1 {
		return ErrInvalidManifestRequeueJitterProbability
	}

	return nil
}

func (f FlagVar) GetWatcherImage() string {
	return fmt.Sprintf("%s/%s:%s", f.WatcherImageRegistry, f.WatcherImageName, f.WatcherImageTag)
}
