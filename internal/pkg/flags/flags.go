package flags

import (
	"errors"
	"flag"
	"os"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	defaultKymaRequeueSuccessInterval                            = 30 * time.Second
	DefaultKymaRequeueErrInterval                                = 2 * time.Second
	DefaultKymaRequeueWarningInterval                            = 30 * time.Second
	DefaultKymaRequeueBusyInterval                               = 5 * time.Second
	defaultManifestRequeueSuccessInterval                        = 30 * time.Second
	defaultMandatoryModuleRequeueSuccessInterval                 = 30 * time.Second
	defaultWatcherRequeueSuccessInterval                         = 30 * time.Second
	defaultClientQPS                                             = 300
	defaultClientBurst                                           = 600
	defaultPprofServerTimeout                                    = 90 * time.Second
	rateLimiterBurstDefault                                      = 200
	rateLimiterFrequencyDefault                                  = 30
	failureBaseDelayDefault                                      = 100 * time.Millisecond
	failureMaxDelayDefault                                       = 5 * time.Second
	defaultCacheSyncTimeout                                      = 2 * time.Minute
	defaultLogLevel                                              = log.WarnLevel
	defaultPurgeFinalizerTimeout                                 = 5 * time.Minute
	defaultMaxConcurrentManifestReconciles                       = 1
	defaultMaxConcurrentKymaReconciles                           = 1
	defaultMaxConcurrentWatcherReconciles                        = 1
	defaultMaxConcurrentMandatoryModulesReconciles               = 1
	DefaultIstioGatewayName                                      = "klm-watcher-gateway"
	DefaultIstioGatewayNamespace                                 = "kcp-system"
	DefaultIstioNamespace                                        = "istio-system"
	DefaultCaCertName                                            = "klm-watcher-serving-cert"
	defaultCaCertCacheTTL                          time.Duration = 1 * time.Hour
	defaultSelfSignedCertDuration                  time.Duration = 90 * 24 * time.Hour
	defaultSelfSignedCertRenewBefore               time.Duration = 60 * 24 * time.Hour
	defaultSelfSignedCertificateRenewBuffer                      = 24 * time.Hour
	DefaultRemoteSyncNamespace                                   = "kyma-system"
)

var (
	errMissingWatcherImageTag = errors.New("runtime watcher image tag is not provided")
	errWatcherDirNotExist     = errors.New("failed to locate watcher resource manifest folder")
)

//nolint:funlen // defines all program flags
func DefineFlagVar() *FlagVar {
	flagVar := new(FlagVar)
	flag.StringVar(&flagVar.MetricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&flagVar.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.StringVar(&flagVar.KymaListenerAddr, "kyma-skr-listener-bind-address", ":8082",
		"The address the skr listener endpoint binds to.")
	flag.StringVar(&flagVar.ManifestListenerAddr, "manifest-skr-listener-bind-address", ":8083",
		"The address the skr listener endpoint binds to.")
	flag.StringVar(&flagVar.PprofAddr, "pprof-bind-address", ":8084",
		"The address the pprof endpoint binds to.")
	flag.IntVar(&flagVar.MaxConcurrentKymaReconciles, "max-concurrent-kyma-reconciles",
		defaultMaxConcurrentKymaReconciles, "The maximum number of concurrent Kyma Reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentManifestReconciles, "max-concurrent-manifest-reconciles",
		defaultMaxConcurrentManifestReconciles,
		"The maximum number of concurrent Manifest Reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentWatcherReconciles, "max-concurrent-watcher-reconciles",
		defaultMaxConcurrentWatcherReconciles,
		"The maximum number of concurrent Watcher Reconciles which can be run.")
	flag.IntVar(&flagVar.MaxConcurrentMandatoryModulesReconciles, "max-concurrent-mandatory-modules-reconciles",
		defaultMaxConcurrentMandatoryModulesReconciles,
		"The maximum number of concurrent Mandatory Modules Reconciles which can be run.")
	flag.BoolVar(&flagVar.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&flagVar.KymaRequeueSuccessInterval, "kyma-requeue-success-interval",
		defaultKymaRequeueSuccessInterval,
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
		defaultMandatoryModuleRequeueSuccessInterval,
		"determines the duration a Kyma in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.ManifestRequeueSuccessInterval, "manifest-requeue-success-interval",
		defaultManifestRequeueSuccessInterval,
		"determines the duration a Manifest in Ready state is enqueued for reconciliation.")
	flag.DurationVar(&flagVar.WatcherRequeueSuccessInterval, "watcher-requeue-success-interval",
		defaultWatcherRequeueSuccessInterval,
		"determines the duration a Watcher in Ready state is enqueued for reconciliation.")

	flag.Float64Var(&flagVar.ClientQPS, "k8s-client-qps", defaultClientQPS, "kubernetes client QPS")
	flag.IntVar(&flagVar.ClientBurst, "k8s-client-burst", defaultClientBurst, "kubernetes client Burst")
	flag.StringVar(&flagVar.ModuleVerificationKeyFilePath, "module-verification-key-file", "",
		"This verification key is used to verify modules against their signature")
	flag.BoolVar(&flagVar.EnableVerification, "enable-verification", false,
		"Enabling verify modules against their signature")
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
	flag.DurationVar(&flagVar.PprofServerTimeout, "pprof-server-timeout", defaultPprofServerTimeout,
		"Timeout of Read / Write for the pprof server.")
	flag.IntVar(&flagVar.RateLimiterBurst, "rate-limiter-burst", rateLimiterBurstDefault,
		"Indicates the rateLimiterBurstDefault value for the bucket rate limiter.")
	flag.IntVar(&flagVar.RateLimiterFrequency, "rate-limiter-frequency", rateLimiterFrequencyDefault,
		"Indicates the bucket rate limiter frequency, signifying no. of events per second.")
	flag.DurationVar(&flagVar.FailureBaseDelay, "failure-base-delay", failureBaseDelayDefault,
		"Indicates the failure base delay in seconds for rate limiter.")
	flag.DurationVar(&flagVar.FailureMaxDelay, "failure-max-delay", failureMaxDelayDefault,
		"Indicates the failure max delay in seconds")
	flag.DurationVar(&flagVar.CacheSyncTimeout, "cache-sync-timeout", defaultCacheSyncTimeout,
		"Indicates the cache sync timeout in seconds")
	flag.BoolVar(&flagVar.EnableDomainNameVerification, "enable-domain-name-pinning", true,
		"Enabling verification of incoming listener request by comparing SAN with KymaCR-SKR-domain")
	flag.IntVar(
		&flagVar.LogLevel, "log-level", defaultLogLevel,
		"indicates the current log-level, enter negative values to increase verbosity (e.g. 9)",
	)
	flag.BoolVar(&flagVar.InKCPMode, "in-kcp-mode", false,
		"Indicates lifecycle manager is deployed in control-plane mode (multiple clusters mode)")
	flag.BoolVar(&flagVar.EnablePurgeFinalizer, "enable-purge-finalizer", false,
		"Enabling purge finalizer")
	flag.DurationVar(&flagVar.PurgeFinalizerTimeout, "purge-finalizer-timeout", defaultPurgeFinalizerTimeout,
		"Indicates the SKR Purge Finalizers execution delay in seconds")
	flag.StringVar(&flagVar.SkipPurgingFor, "skip-finalizer-purging-for", "", "Exclude the passed CRDs"+
		" from finalizer removal. Example: 'ingressroutetcps.traefik.containo.us,*.helm.cattle.io'.")
	flag.StringVar(&flagVar.RemoteSyncNamespace, "sync-namespace", DefaultRemoteSyncNamespace,
		"Name of the namespace for syncing remote Kyma and module catalog")
	flag.StringVar(&flagVar.CaCertName, "ca-cert-name", DefaultCaCertName,
		"Name of the CA Certificate in Istio Namespace which is used to sign SKR Certificates")
	flag.DurationVar(&flagVar.CaCertCacheTTL, "ca-cert-cache-ttl", defaultCaCertCacheTTL,
		"The ttl for the CA Certificate Cache")
	flag.DurationVar(&flagVar.SelfSignedCertDuration, "self-signed-cert-duration", defaultSelfSignedCertDuration,
		"The lifetime duration of self-signed certificate, minimum accepted duration is 1 hour.")
	flag.DurationVar(&flagVar.SelfSignedCertRenewBefore, "self-signed-cert-renew-before",
		defaultSelfSignedCertRenewBefore,
		"How long before the currently issued self-signed certificate's expiry cert-manager should renew the certificate")
	flag.DurationVar(&flagVar.SelfSignedCertRenewBuffer, "self-signed-cert-renew-buffer",
		defaultSelfSignedCertificateRenewBuffer,
		"The buffer duration to wait before confirm self-signed certificate not renewed")
	flag.BoolVar(&flagVar.IsKymaManaged, "is-kyma-managed", false, "indicates whether Kyma is managed")
	flag.StringVar(&flagVar.DropStoredVersion, "drop-stored-version", "v1alpha1",
		"The API version to be dropped from the storage versions")
	flag.StringVar(&flagVar.WatcherImageTag, "skr-watcher-image-tag", "",
		`Image tag to be used for the SKR watcher image.`)
	flag.BoolVar(&flagVar.UseWatcherDevRegistry, "watcher-dev-registry", false,
		`Enable to use the dev registry for fetching the watcher image.`)
	flag.StringVar(&flagVar.WatcherResourceLimitsMemory, "skr-webhook-memory-limits", "200Mi",
		"The resources.limits.memory for skr webhook.")
	flag.StringVar(&flagVar.WatcherResourceLimitsCPU, "skr-webhook-cpu-limits", "0.1",
		"The resources.limits.cpu for skr webhook.")
	flag.StringVar(&flagVar.WatcherResourcesPath, "skr-watcher-path", "./skr-webhook",
		"The path to the skr watcher resources.")
	return flagVar
}

type FlagVar struct {
	MetricsAddr                             string
	EnableDomainNameVerification            bool
	EnableLeaderElection                    bool
	EnablePurgeFinalizer                    bool
	EnableKcpWatcher                        bool
	EnableWebhooks                          bool
	ProbeAddr                               string
	KymaListenerAddr, ManifestListenerAddr  string
	MaxConcurrentKymaReconciles             int
	MaxConcurrentManifestReconciles         int
	MaxConcurrentWatcherReconciles          int
	MaxConcurrentMandatoryModulesReconciles int
	KymaRequeueSuccessInterval              time.Duration
	KymaRequeueErrInterval                  time.Duration
	KymaRequeueBusyInterval                 time.Duration
	KymaRequeueWarningInterval              time.Duration
	ManifestRequeueSuccessInterval          time.Duration
	WatcherRequeueSuccessInterval           time.Duration
	MandatoryModuleRequeueSuccessInterval   time.Duration
	ModuleVerificationKeyFilePath           string
	ClientQPS                               float64
	ClientBurst                             int
	IstioNamespace                          string
	IstioGatewayName                        string
	IstioGatewayNamespace                   string
	AdditionalDNSNames                      string
	// ListenerPortOverwrite is used to enable the user to overwrite the port
	// used to expose the KCP cluster for the watcher. By default, it will be
	// fetched from the specified gateway.
	ListenerPortOverwrite                  string
	Pprof                                  bool
	PprofAddr                              string
	PprofServerTimeout                     time.Duration
	FailureBaseDelay, FailureMaxDelay      time.Duration
	RateLimiterBurst, RateLimiterFrequency int
	CacheSyncTimeout                       time.Duration
	LogLevel                               int
	InKCPMode                              bool
	PurgeFinalizerTimeout                  time.Duration
	SkipPurgingFor                         string
	RemoteSyncNamespace                    string
	CaCertName                             string
	CaCertCacheTTL                         time.Duration
	EnableVerification                     bool
	IsKymaManaged                          bool
	SelfSignedCertDuration                 time.Duration
	SelfSignedCertRenewBefore              time.Duration
	SelfSignedCertRenewBuffer              time.Duration
	DropStoredVersion                      string
	UseWatcherDevRegistry                  bool
	WatcherImageTag                        string
	WatcherResourceLimitsMemory            string
	WatcherResourceLimitsCPU               string
	WatcherResourcesPath                   string
}

func (f FlagVar) Validate() error {
	if f.EnableKcpWatcher {
		if f.WatcherImageTag == "" {
			return errMissingWatcherImageTag
		}
		dirInfo, err := os.Stat(f.WatcherResourcesPath)
		if err != nil || !dirInfo.IsDir() {
			return errWatcherDirNotExist
		}
	}

	return nil
}
