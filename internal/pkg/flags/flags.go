package flags

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	DefaultKymaRequeueSuccessInterval                                   = 5 * time.Minute
	DefaultKymaRequeueErrInterval                                       = 2 * time.Second
	DefaultKymaRequeueWarningInterval                                   = 30 * time.Second
	DefaultKymaRequeueBusyInterval                                      = 5 * time.Second
	DefaultManifestRequeueSuccessInterval                               = 5 * time.Minute
	DefaultManifestRequeueErrInterval                                   = 2 * time.Second
	DefaultManifestRequeueWarningInterval                               = 30 * time.Second
	DefaultManifestRequeueBusyInterval                                  = 5 * time.Second
	DefaultManifestRequeueJitterProbability                             = 0.02
	DefaultManifestRequeueJitterPercentage                              = 0.02
	DefaultMandatoryModuleRequeueSuccessInterval                        = 30 * time.Second
	DefaultMandatoryModuleDeletionRequeueSuccessInterval                = 30 * time.Second
	DefaultWatcherRequeueSuccessInterval                                = 1 * time.Minute
	DefaultClientQPS                                                    = 1000
	DefaultClientBurst                                                  = 2000
	DefaultPprofServerTimeout                                           = 90 * time.Second
	RateLimiterBurstDefault                                             = 2000
	RateLimiterFrequencyDefault                                         = 1000
	FailureBaseDelayDefault                                             = 5 * time.Second
	FailureMaxDelayDefault                                              = 30 * time.Second
	DefaultCacheSyncTimeout                                             = 60 * time.Minute
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
	DefaultSelfSignedCertIssuerNamespace                                = "istio-system"
	DefaultSelfSignedCertDuration                         time.Duration = 60 * 24 * time.Hour
	DefaultSelfSignedCertRenewBefore                      time.Duration = 30 * 24 * time.Hour
	DefaultSelfSignedCertificateRenewBuffer                             = 24 * time.Hour
	DefaultSelfSignedCertKeySize                                        = 4096
	DefaultSelfSignedCertificateIssuerName                              = "klm-watcher-selfsigned"
	DefaultIstioGatewayCertSwitchBeforeExpirationTime                   = 33 * 24 * time.Hour
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
	DefaultDropCrdStoredVersionMap                                      = "Manifest:v1beta1,Watcher:v1beta1,ModuleTemplate:v1beta1,Kyma:v1beta1" //nolint:revive // keep it readible
	DefaultMetricsCleanupIntervalInMinutes                              = 15
	DefaultMinMaintenanceWindowSize                                     = 20 * time.Minute
	DefaultLeaderElectionLeaseDuration                                  = 180 * time.Second
	DefaultLeaderElectionRenewDeadline                                  = 120 * time.Second
	DefaultLeaderElectionRetryPeriod                                    = 3 * time.Second
)

var (
	ErrMissingWatcherImageTag      = errors.New("runtime watcher image tag is not provided")
	ErrMissingWatcherImageRegistry = errors.New("runtime watcher image registry is not provided")
	ErrWatcherDirNotExist          = errors.New("failed to locate watcher resource manifest folder")
	ErrLeaderElectionTimeoutConfig = errors.New(
		"configured leader-election-renew-deadline must be less than leader-election-lease-duration",
	)
	ErrInvalidSelfSignedCertKeyLength         = errors.New("invalid self-signed-cert-key-size: must be 4096")
	ErrInvalidManifestRequeueJitterPercentage = errors.New(
		"invalid manifest requeue jitter percentage: must be between 0 and 0.05",
	)
	ErrInvalidManifestRequeueJitterProbability = errors.New(
		"invalid manifest requeue jitter probability: must be between 0 and 1",
	)
)

//nolint:funlen // defines all program flags
func DefineFlagVar() *FlagVar {
	flagVar := new(FlagVar)
	flag.StringVar(&flagVar.CertificateManagement, "cert-management", certmanagerv1.SchemeGroupVersion.String(),
		fmt.Sprintf("Certificate management system to use. Accepted values: '%s', '%s'. Default: '%s'",
			certmanagerv1.SchemeGroupVersion.String(),
			gcertv1alpha1.SchemeGroupVersion.String(),
			certmanagerv1.SchemeGroupVersion.String()))
	flag.StringVar(&flagVar.MetricsAddr, "metrics-bind-address", DefaultMetricsAddress,
		"Address and port for binding of metrics endpoint.")
	flag.StringVar(&flagVar.ProbeAddr, "health-probe-bind-address", DefaultProbeAddress,
		"Address and port for binding of health probe endpoint.")
	flag.StringVar(&flagVar.KymaListenerAddr, "kyma-skr-listener-bind-address", DefaultKymaListenerAddress,
		"Address and port for binding the SKR event listener for Kyma resources.")
	flag.StringVar(&flagVar.ManifestListenerAddr, "manifest-skr-listener-bind-address", DefaultManifestListenerAddress,
		"Address and port for binding the SKR event listener for Manifest resources.")
	flag.StringVar(&flagVar.PprofAddr, "pprof-bind-address", DefaultPprofAddress,
		"Address and port for binding of pprof profiling endpoint.")
	flag.IntVar(&flagVar.MaxConcurrentKymaReconciles, "max-concurrent-kyma-reconciles",
		DefaultMaxConcurrentKymaReconciles, "Maximum number of concurrent Kyma reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentManifestReconciles, "max-concurrent-manifest-reconciles",
		DefaultMaxConcurrentManifestReconciles,
		"Maximum number of concurrent Manifest reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentWatcherReconciles, "max-concurrent-watcher-reconciles",
		DefaultMaxConcurrentWatcherReconciles,
		"Maximum number of concurrent Watcher reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentMandatoryModuleReconciles, "max-concurrent-mandatory-modules-reconciles",
		DefaultMaxConcurrentMandatoryModuleReconciles,
		"Maximum number of concurrent Mandatory Modules installation reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentMandatoryModuleDeletionReconciles,
		"max-concurrent-mandatory-modules-deletion-reconciles",
		DefaultMaxConcurrentMandatoryModuleDeletionReconciles,
		"Maximum number of concurrent Mandatory Modules deletion reconciles which can be run.")
	flag.BoolVar(&flagVar.EnableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&flagVar.LeaderElectionLeaseDuration, "leader-election-lease-duration",
		DefaultLeaderElectionLeaseDuration,
		"Duration configured for the 'LeaseDuration' option of the controller-runtime library"+
			" used to run the controller manager process.")
	flag.DurationVar(&flagVar.LeaderElectionRenewDeadline, "leader-election-renew-deadline",
		DefaultLeaderElectionRenewDeadline,
		"Duration configured for the 'RenewDeadline' option of the controller-runtime library "+
			"used to run the controller manager process.")
	flag.DurationVar(&flagVar.LeaderElectionRetryPeriod, "leader-election-retry-period",
		DefaultLeaderElectionRetryPeriod,
		"Duration configured for the 'RetryPeriod' option of the controller-runtime library"+
			" used to run the controller manager process.")
	flag.DurationVar(&flagVar.KymaRequeueSuccessInterval, "kyma-requeue-success-interval",
		DefaultKymaRequeueSuccessInterval,
		"Duration after which a Kyma in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.KymaRequeueErrInterval, "kyma-requeue-error-interval",
		DefaultKymaRequeueErrInterval,
		"Duration after which a Kyma in Error state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.KymaRequeueWarningInterval, "kyma-requeue-warning-interval",
		DefaultKymaRequeueWarningInterval,
		"Duration after which a Kyma in Warning state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.KymaRequeueBusyInterval, "kyma-requeue-busy-interval",
		DefaultKymaRequeueBusyInterval,
		"Duration after which a Kyma in Processing state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.MandatoryModuleRequeueSuccessInterval, "mandatory-module-requeue-success-interval",
		DefaultMandatoryModuleRequeueSuccessInterval,
		"Duration after which a Kyma in Ready state is enqueued for mandatory module installation reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueSuccessInterval, "manifest-requeue-success-interval",
		DefaultManifestRequeueSuccessInterval,
		"Duration after which a Manifest in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueErrInterval, "manifest-requeue-error-interval",
		DefaultManifestRequeueErrInterval,
		"Duration after which a Manifest in Error state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueWarningInterval, "manifest-requeue-warning-interval",
		DefaultManifestRequeueWarningInterval,
		"Duration after which a Manifest in Warning state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueBusyInterval, "manifest-requeue-busy-interval",
		DefaultManifestRequeueBusyInterval,
		"Duration after which a Manifest in Processing state is enqueued for reconciliation.")
	flag.Float64Var(&flagVar.ManifestRequeueJitterProbability, "manifest-requeue-jitter-probability",
		DefaultManifestRequeueJitterProbability,
		"Percentage probability that jitter is applied to the requeue interval. Used for all controllers.")
	flag.Float64Var(&flagVar.ManifestRequeueJitterPercentage, "manifest-requeue-jitter-percentage",
		DefaultManifestRequeueJitterPercentage,
		"Percentage range for the jitter applied to the requeue interval. "+
			"E.g., 0.1 means +/- 10% of the interval.")
	flag.DurationVar(&flagVar.MandatoryModuleDeletionRequeueSuccessInterval,
		"mandatory-module-deletion-requeue-success-interval",
		DefaultMandatoryModuleDeletionRequeueSuccessInterval,
		"Duration after which a Kyma in Ready state is enqueued for mandatory module deletion reconciliation.")
	flag.DurationVar(&flagVar.WatcherRequeueSuccessInterval, "watcher-requeue-success-interval",
		DefaultWatcherRequeueSuccessInterval,
		"Duration after which a Watcher in Ready state is enqueued for reconciliation.")

	flag.Float64Var(&flagVar.ClientQPS, "k8s-client-qps", DefaultClientQPS,
		"Maximum queries per second (QPS) limit for the Kubernetes client. Controls how many requests"+
			" can be made to the Kubernetes API server per second in steady state.")
	flag.IntVar(&flagVar.ClientBurst, "k8s-client-burst", DefaultClientBurst,
		"Maximum burst size for throttling Kubernetes API requests. Allows temporarily exceeding the QPS"+
			" limit when there are sudden spikes in request volume.")
	flag.BoolVar(&flagVar.EnableWebhooks, "enable-webhooks", false,
		"Enable Validation/Conversion Webhooks.")
	flag.BoolVar(&flagVar.EnableKcpWatcher, "enable-kcp-watcher", true,
		"Enable KCP Watcher controller to reconcile Watcher CRs.")
	flag.StringVar(&flagVar.AdditionalDNSNames, "additional-dns-names", "",
		"Additional DNS Names which are added to SKR certificates as SANs. Input should be given as "+
			"comma-separated list, for example \"--additional-dns-names=localhost,127.0.0.1,host.k3d.internal\".")
	flag.StringVar(&flagVar.IstioNamespace, "istio-namespace", DefaultIstioNamespace,
		"Namespace of Istio resources in cluster.")
	flag.StringVar(&flagVar.IstioGatewayName, "istio-gateway-name", DefaultIstioGatewayName,
		"Name of Istio Gateway resource in cluster.")
	flag.StringVar(&flagVar.IstioGatewayNamespace, "istio-gateway-namespace", DefaultIstioGatewayNamespace,
		"Namespace of the Istio Gateway resource in cluster.")
	flag.StringVar(&flagVar.ListenerPortOverwrite, "listener-port-overwrite", "",
		"Port that is mapped to HTTP port of the local k3d cluster using --port 9443:443@loadbalancer when "+
			"creating the KCP cluster.")
	flag.BoolVar(&flagVar.Pprof, "pprof", false, "Enable a pprof server.")
	flag.DurationVar(&flagVar.PprofServerTimeout, "pprof-server-timeout", DefaultPprofServerTimeout,
		"Duration of timeout of read/write for the pprof server.")
	flag.IntVar(&flagVar.RateLimiterBurst, "rate-limiter-burst", RateLimiterBurstDefault,
		"Maximum number of requests that can be processed immediately by all controllers before being rate limited. "+
			"Controls how many reconciliations can happen in quick succession during burst periods.")
	flag.IntVar(&flagVar.RateLimiterFrequency, "rate-limiter-frequency", RateLimiterFrequencyDefault,
		"Number of requests per second added to the rate limiter bucket for all controllers. "+
			"Controls how many reconciliations are allowed to happen in steady state.")
	flag.DurationVar(&flagVar.FailureBaseDelay, "failure-base-delay", FailureBaseDelayDefault,
		"Duration of the failure base delay for rate limiting in all controllers.")
	flag.DurationVar(&flagVar.FailureMaxDelay, "failure-max-delay", FailureMaxDelayDefault,
		"Duration of the failure max delay for rate limiting in all controllers.")
	flag.DurationVar(&flagVar.CacheSyncTimeout, "cache-sync-timeout", DefaultCacheSyncTimeout,
		"Duration of the cache sync timeout in all controllers.")
	flag.BoolVar(&flagVar.EnableDomainNameVerification, "enable-domain-name-pinning", true,
		"Enable verification of incoming listener request by comparing SAN with SKR domain in Kyma CR.")
	flag.IntVar(
		&flagVar.LogLevel, "log-level", DefaultLogLevel,
		"Log level. Enter negative or positive values to increase verbosity. 0 has the lowest verbosity.",
	)
	flag.BoolVar(&flagVar.EnablePurgeFinalizer, "enable-purge-finalizer", true,
		"Enable Purge controller.")
	flag.DurationVar(&flagVar.PurgeFinalizerTimeout, "purge-finalizer-timeout", DefaultPurgeFinalizerTimeout,
		"Duration after a Kyma's deletion timestamp when the remaining resources should be purged in the SKR.")
	flag.StringVar(&flagVar.SkipPurgingFor, "skip-finalizer-purging-for", "", "CRDs to be excluded "+
		"from finalizer removal. Example: 'ingressroutetcps.traefik.containo.us,*.helm.cattle.io'.")
	flag.StringVar(&flagVar.RemoteSyncNamespace, "sync-namespace", DefaultRemoteSyncNamespace,
		"Namespace for syncing remote Kyma and module catalog.")
	flag.DurationVar(&flagVar.SelfSignedCertDuration, "self-signed-cert-duration", DefaultSelfSignedCertDuration,
		"Duration of self-signed certificate. Minimum: 1h.")
	flag.DurationVar(&flagVar.SelfSignedCertRenewBefore, "self-signed-cert-renew-before",
		DefaultSelfSignedCertRenewBefore,
		"Duration before the currently issued self-signed certificate's expiry "+
			"when cert-manager should renew the certificate.")
	flag.DurationVar(&flagVar.SelfSignedCertRenewBuffer, "self-signed-cert-renew-buffer",
		DefaultSelfSignedCertificateRenewBuffer,
		"Duration to wait before confirm self-signed certificate not renewed.")
	flag.IntVar(&flagVar.SelfSignedCertKeySize, "self-signed-cert-key-size", DefaultSelfSignedCertKeySize,
		"Key size for the self-signed certificate.")
	flag.StringVar(&flagVar.SelfSignedCertificateIssuerName, "self-signed-cert-issuer-name",
		DefaultSelfSignedCertificateIssuerName, "Issuer name for the self-signed certificate.")
	flag.DurationVar(&flagVar.IstioGatewayCertSwitchBeforeExpirationTime,
		"istio-gateway-cert-switch-before-expiration-time", DefaultIstioGatewayCertSwitchBeforeExpirationTime,
		"Duration before the expiration of the current CA certificate when the Gateway certificate should be switched.")
	flag.StringVar(&flagVar.SelfSignedCertIssuerNamespace, "self-signed-cert-issuer-namespace",
		DefaultSelfSignedCertIssuerNamespace,
		"Namespace of the Issuer for self-signed certificates.")
	flag.DurationVar(&flagVar.IstioGatewaySecretRequeueSuccessInterval,
		"istio-gateway-secret-requeue-success-interval", DefaultIstioGatewaySecretRequeueSuccessInterval,
		"Duration after which the Istio Gateway Secret is enqueued after successful reconciliation.")
	flag.DurationVar(&flagVar.IstioGatewaySecretRequeueErrInterval,
		"istio-gateway-secret-requeue-error-interval", DefaultIstioGatewaySecretRequeueErrInterval,
		"Duration after which the Istio Gateway Secret is enqueued after unsuccessful reconciliation.")
	flag.BoolVar(&flagVar.UseLegacyStrategyForIstioGatewaySecret, "legacy-strategy-for-istio-gateway-secret",
		false, "Use the legacy strategy (with downtime) for the Istio Gateway Secret.")
	flag.BoolVar(&flagVar.IsKymaManaged, "is-kyma-managed", true, "Use managed Kyma mode.")
	flag.StringVar(&flagVar.DropCrdStoredVersionMap, "drop-crd-stored-version-map", DefaultDropCrdStoredVersionMap,
		"API versions to be dropped from the storage version. The input format should be a "+
			"comma-separated list of API versions, where each API version is in the format 'kind:version'.")
	flag.StringVar(&flagVar.WatcherImageName, "skr-watcher-image-name", DefaultWatcherImageName,
		`Image name to be used for the SKR Watcher image.`)
	flag.StringVar(&flagVar.WatcherImageTag, "skr-watcher-image-tag", "",
		`Image tag to be used for the SKR Watcher image.`)
	flag.StringVar(&flagVar.WatcherImageRegistry, "skr-watcher-image-registry", DefaultWatcherImageRegistry,
		`Image registry to be used for the SKR Watcher image.`)
	flag.StringVar(&flagVar.WatcherResourceLimitsMemory, "skr-webhook-memory-limits",
		DefaultWatcherResourceLimitsMemory,
		"Resource limit for memory allocation to the SKR webhook.")
	flag.StringVar(&flagVar.WatcherResourceLimitsCPU, "skr-webhook-cpu-limits", DefaultWatcherResourceLimitsCPU,
		"Resource limit for CPU allocation to the SKR webhook.")
	flag.StringVar(&flagVar.WatcherResourcesPath, "skr-watcher-path", DefaultWatcherResourcesPath,
		"Path to the static SKR Watcher resources.")
	flag.IntVar(&flagVar.MetricsCleanupIntervalInMinutes, "metrics-cleanup-interval",
		DefaultMetricsCleanupIntervalInMinutes,
		"Interval (in minutes) at which the cleanup of non-existing Kyma CRs metrics runs.")
	flag.DurationVar(&flagVar.MinMaintenanceWindowSize, "min-maintenance-window-size",
		DefaultMinMaintenanceWindowSize,
		"Minimum duration of maintenance window required for reconciling modules with downtime.")
	flag.StringVar(&flagVar.OciRegistryCredSecretName, "oci-registry-cred-secret", "",
		"Allows to configure name of the Secret containing the OCI registry credential")
	flag.StringVar(&flagVar.OciRegistryHost, "oci-registry-host", "",
		"Allows to configure hostname of the OCI registry.")
	return flagVar
}

type FlagVar struct {
	CertificateManagement                          string
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
	PurgeFinalizerTimeout                      time.Duration
	SkipPurgingFor                             string
	RemoteSyncNamespace                        string
	IsKymaManaged                              bool
	SelfSignedCertDuration                     time.Duration
	SelfSignedCertRenewBefore                  time.Duration
	SelfSignedCertRenewBuffer                  time.Duration
	SelfSignedCertKeySize                      int
	SelfSignedCertIssuerNamespace              string
	SelfSignedCertificateIssuerName            string
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
	OciRegistryCredSecretName                  string
	OciRegistryHost                            string
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
		2048: false, // 2048 is a valid value for cert-manager,
		// but explicitly prohibited as not compliant to security requirements
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

	if !map[string]bool{
		certmanagerv1.SchemeGroupVersion.String(): true,
		gcertv1alpha1.SchemeGroupVersion.String(): true,
	}[f.CertificateManagement] {
		return fmt.Errorf("%w: '%s'", common.ErrUnsupportedCertificateManagementSystem, f.CertificateManagement)
	}

	if err := validateOciRegistryConfig(f.OciRegistryHost, f.OciRegistryCredSecretName); err != nil {
		return err
	}

	return nil
}

func validateOciRegistryConfig(host, credSecretName string) error {
	if host == "" && credSecretName == "" {
		return common.ErrNoOCIRegistryHostAndCredSecret
	}
	if host != "" && credSecretName != "" {
		return common.ErrBothOCIRegistryHostAndCredSecretProvided
	}
	return nil
}

func (f FlagVar) GetWatcherImage() string {
	return fmt.Sprintf("%s/%s:%s", f.WatcherImageRegistry, f.WatcherImageName, f.WatcherImageTag)
}
