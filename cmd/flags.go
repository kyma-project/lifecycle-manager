package main

import (
	"flag"
	"time"

	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	defaultKymaRequeueSuccessInterval      = 30 * time.Second
	defaultKymaRequeueErrInterval          = 2 * time.Second
	defaultKymaRequeueBusyInterval         = 5 * time.Second
	defaultManifestRequeueSuccessInterval  = 30 * time.Second
	defaultWatcherRequeueSuccessInterval   = 30 * time.Second
	defaultClientQPS                       = 300
	defaultClientBurst                     = 600
	defaultPprofServerTimeout              = 90 * time.Second
	rateLimiterBurstDefault                = 200
	rateLimiterFrequencyDefault            = 30
	failureBaseDelayDefault                = 100 * time.Millisecond
	failureMaxDelayDefault                 = 5 * time.Second
	defaultCacheSyncTimeout                = 2 * time.Minute
	defaultLogLevel                        = log.WarnLevel
	defaultPurgeFinalizerTimeout           = 5 * time.Minute
	defaultMaxConcurrentManifestReconciles = 1
	defaultMaxConcurrentKymaReconciles     = 1
	defaultMaxConcurrentWatcherReconciles  = 1
	defaultIstioGatewayName                = "lifecycle-manager-watcher-gateway"
	defaultIstioGatewayNamespace           = "kcp-system"
	defaultIstioNamespace                  = "istio-system"
)

//nolint:funlen
func defineFlagVar() *FlagVar {
	flagVar := new(FlagVar)
	flag.StringVar(&flagVar.metricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&flagVar.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.StringVar(&flagVar.kymaListenerAddr, "kyma-skr-listener-bind-address", ":8082",
		"The address the skr listener endpoint binds to.")
	flag.StringVar(&flagVar.manifestListenerAddr, "manifest-skr-listener-bind-address", ":8083",
		"The address the skr listener endpoint binds to.")
	flag.StringVar(&flagVar.pprofAddr, "pprof-bind-address", ":8084",
		"The address the pprof endpoint binds to.")
	flag.IntVar(&flagVar.maxConcurrentKymaReconciles, "max-concurrent-kyma-reconciles",
		defaultMaxConcurrentKymaReconciles, "The maximum number of concurrent Kyma Reconciles which can be run.")
	flag.IntVar(&flagVar.maxConcurrentManifestReconciles, "max-concurrent-manifest-reconciles",
		defaultMaxConcurrentManifestReconciles,
		"The maximum number of concurrent Manifest Reconciles which can be run.")
	flag.IntVar(&flagVar.maxConcurrentWatcherReconciles, "max-concurrent-watcher-reconciles",
		defaultMaxConcurrentWatcherReconciles,
		"The maximum number of concurrent Watcher Reconciles which can be run.")
	flag.BoolVar(&flagVar.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&flagVar.kymaRequeueSuccessInterval, "kyma-requeue-success-interval",
		defaultKymaRequeueSuccessInterval, "determines the duration a Kyma in Ready state is enqueued for reconciliation.") //nolint:lll
	flag.DurationVar(&flagVar.kymaRequeueErrInterval, "kyma-requeue-error-interval",
		defaultKymaRequeueErrInterval, "determines the duration after which a Kyma in Error state is enqueued for reconciliation.") //nolint:lll
	flag.DurationVar(&flagVar.kymaRequeueBusyInterval, "kyma-requeue-busy-interval",
		defaultKymaRequeueBusyInterval, "determines the duration after which a Kyma in Processing state is enqueued for reconciliation.") //nolint:lll
	flag.DurationVar(&flagVar.manifestRequeueSuccessInterval, "manifest-requeue-success-interval",
		defaultManifestRequeueSuccessInterval, "determines the duration a Manifest in Ready state is enqueued for reconciliation.") //nolint:lll
	flag.DurationVar(&flagVar.watcherRequeueSuccessInterval, "watcher-requeue-success-interval",
		defaultWatcherRequeueSuccessInterval, "determines the duration a Watcher in Ready state is enqueued for reconciliation.") //nolint:lll

	flag.Float64Var(&flagVar.clientQPS, "k8s-client-qps", defaultClientQPS, "kubernetes client QPS")
	flag.IntVar(&flagVar.clientBurst, "k8s-client-burst", defaultClientBurst, "kubernetes client Burst")
	flag.StringVar(&flagVar.moduleVerificationKeyFilePath, "module-verification-key-file", "",
		"This verification key is used to verify modules against their signature")
	flag.BoolVar(&flagVar.enableVerification, "enable-verification", false,
		"Enabling verify modules against their signature")
	flag.BoolVar(&flagVar.enableWebhooks, "enable-webhooks", false,
		"Enabling Validation/Conversion Webhooks.")
	flag.BoolVar(&flagVar.enableKcpWatcher, "enable-kcp-watcher", false,
		"Enabling KCP Watcher to reconcile Watcher CRs created by KCP run operators")
	flag.StringVar(&flagVar.skrWatcherPath, "skr-watcher-path", "./skr-webhook",
		"The path to the skr watcher resources.")
	flag.StringVar(&flagVar.skrWebhookMemoryLimits, "skr-webhook-memory-limits", "200Mi",
		"The resources.limits.memory for skr webhook.")
	flag.StringVar(&flagVar.skrWebhookCPULimits, "skr-webhook-cpu-limits", "0.1",
		"The resources.limits.cpu for skr webhook.")
	flag.StringVar(&flagVar.additionalDNSNames, "additional-dns-names", "",
		"Additional DNS Names which are added to Kyma Certificates as SANs. Input should be given as "+
			"comma-separated list, for example \"--additional-dns-names=localhost,127.0.0.1,host.k3d.internal\".")
	flag.StringVar(&flagVar.istioNamespace, "istio-namespace", defaultIstioNamespace,
		"Cluster Resource Namespace of Istio")
	flag.StringVar(&flagVar.istioGatewayName, "istio-gateway-name", defaultIstioGatewayName,
		"Cluster Resource Name of Istio Gateway")
	flag.StringVar(&flagVar.istioGatewayNamespace, "istio-gateway-namespace", defaultIstioGatewayNamespace,
		"Cluster Resource Namespace of Istio Gateway")
	flag.StringVar(&flagVar.listenerPortOverwrite, "listener-port-overwrite", "",
		"Port that is mapped to HTTP port of the local k3d cluster using --port 9443:443@loadbalancer when "+
			"creating the KCP cluster")
	flag.StringVar(&flagVar.skrWatcherImage, "skr-watcher-image", "runtime-watcher-skr:v20230824-f4459bad",
		`Image of the SKR watcher.`)
	flag.BoolVar(&flagVar.pprof, "pprof", false, "Whether to start up a pprof server.")
	flag.DurationVar(&flagVar.pprofServerTimeout, "pprof-server-timeout", defaultPprofServerTimeout,
		"Timeout of Read / Write for the pprof server.")
	flag.IntVar(&flagVar.rateLimiterBurst, "rate-limiter-burst", rateLimiterBurstDefault,
		"Indicates the rateLimiterBurstDefault value for the bucket rate limiter.")
	flag.IntVar(&flagVar.rateLimiterFrequency, "rate-limiter-frequency", rateLimiterFrequencyDefault,
		"Indicates the bucket rate limiter frequency, signifying no. of events per second.")
	flag.DurationVar(&flagVar.failureBaseDelay, "failure-base-delay", failureBaseDelayDefault,
		"Indicates the failure base delay in seconds for rate limiter.")
	flag.DurationVar(&flagVar.failureMaxDelay, "failure-max-delay", failureMaxDelayDefault,
		"Indicates the failure max delay in seconds")
	flag.DurationVar(&flagVar.cacheSyncTimeout, "cache-sync-timeout", defaultCacheSyncTimeout,
		"Indicates the cache sync timeout in seconds")
	flag.BoolVar(&flagVar.enableDomainNameVerification, "enable-domain-name-pinning", true,
		"Enabling verification of incoming listener request by comparing SAN with KymaCR-SKR-domain")
	flag.IntVar(
		&flagVar.logLevel, "log-level", defaultLogLevel,
		"indicates the current log-level, enter negative values to increase verbosity (e.g. 9)",
	)
	flag.BoolVar(&flagVar.inKCPMode, "in-kcp-mode", false,
		"Indicates lifecycle manager is deployed in control-plane mode (multiple clusters mode)")
	flag.BoolVar(&flagVar.enablePurgeFinalizer, "enable-purge-finalizer", false,
		"Enabling purge finalizer")
	flag.DurationVar(&flagVar.purgeFinalizerTimeout, "purge-finalizer-timeout", defaultPurgeFinalizerTimeout,
		"Indicates the SKR Purge Finalizers execution delay in seconds")
	flag.StringVar(&flagVar.skipPurgingFor, "skip-finalizer-purging-for", "", "Exclude the passed CRDs"+
		" from finalizer removal. Example: 'ingressroutetcps.traefik.containo.us,*.helm.cattle.io'.")
	flag.StringVar(&flagVar.remoteSyncNamespace, "sync-namespace", controller.DefaultRemoteSyncNamespace,
		"Name of the namespace for syncing remote Kyma and module catalog")
	flag.BoolVar(&flagVar.isKymaManaged, "is-kyma-managed", false, "indicates whether Kyma is managed")
	return flagVar
}

type FlagVar struct {
	metricsAddr                            string
	enableLeaderElection                   bool
	probeAddr                              string
	kymaListenerAddr, manifestListenerAddr string
	maxConcurrentKymaReconciles            int
	maxConcurrentManifestReconciles        int
	maxConcurrentWatcherReconciles         int
	kymaRequeueSuccessInterval             time.Duration
	kymaRequeueErrInterval                 time.Duration
	kymaRequeueBusyInterval                time.Duration
	manifestRequeueSuccessInterval         time.Duration
	watcherRequeueSuccessInterval          time.Duration
	moduleVerificationKeyFilePath          string
	clientQPS                              float64
	clientBurst                            int
	enableWebhooks                         bool
	enableKcpWatcher                       bool
	skrWatcherPath                         string
	skrWebhookMemoryLimits                 string
	skrWebhookCPULimits                    string
	istioNamespace                         string
	istioGatewayName                       string
	istioGatewayNamespace                  string
	additionalDNSNames                     string
	// listenerPortOverwrite is used to enable the user to overwrite the port
	// used to expose the KCP cluster for the watcher. By default, it will be
	// fetched from the specified gateway.
	listenerPortOverwrite                  string
	skrWatcherImage                        string
	pprof                                  bool
	pprofAddr                              string
	pprofServerTimeout                     time.Duration
	failureBaseDelay, failureMaxDelay      time.Duration
	rateLimiterBurst, rateLimiterFrequency int
	cacheSyncTimeout                       time.Duration
	enableDomainNameVerification           bool
	logLevel                               int
	inKCPMode                              bool
	enablePurgeFinalizer                   bool
	purgeFinalizerTimeout                  time.Duration
	skipPurgingFor                         string
	remoteSyncNamespace                    string
	enableVerification                     bool
	isKymaManaged                          bool
}
