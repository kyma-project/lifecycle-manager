package main

import (
	"flag"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	defaultKymaRequeueSuccessInterval    = 20 * time.Second
	defaultWatcherRequeueSuccessInterval = 20 * time.Second
	defaultClientQPS                     = 150
	defaultClientBurst                   = 150
	defaultPprofServerTimeout            = 90 * time.Second
	rateLimiterBurstDefault              = 200
	rateLimiterFrequencyDefault          = 30
	failureBaseDelayDefault              = 100 * time.Millisecond
	failureMaxDelayDefault               = 1000 * time.Second
	defaultCacheSyncTimeout              = 2 * time.Minute
	defaultListenerPort                  = 9080
	defaultLogLevel                      = log.WarnLevel
)

//nolint:funlen
func defineFlagVar() *FlagVar {
	flagVar := new(FlagVar)
	flag.StringVar(&flagVar.metricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&flagVar.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.StringVar(&flagVar.listenerAddr, "skr-listener-bind-address", ":8082",
		"The address the skr listener endpoint binds to.")
	flag.StringVar(&flagVar.pprofAddr, "pprof-bind-address", ":8083",
		"The address the pprof endpoint binds to.")
	flag.IntVar(&flagVar.maxConcurrentReconciles, "max-concurrent-reconciles", 1,
		"The maximum number of concurrent Reconciles which can be run.")
	flag.BoolVar(&flagVar.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&flagVar.kymaRequeueSuccessInterval, "kyma-requeue-success-interval",
		defaultKymaRequeueSuccessInterval, "determines the duration after which an already successfully "+
			"reconciled Kyma is enqueued for checking if it's still in a consistent state.")
	flag.DurationVar(&flagVar.watcherRequeueSuccessInterval, "watcher-requeue-success-interval",
		defaultWatcherRequeueSuccessInterval, "determines the duration after which an already successfully "+
			"reconciled watcher is enqueued for checking if it's still in a consistent state.")
	flag.Float64Var(&flagVar.clientQPS, "k8s-client-qps", defaultClientQPS, "kubernetes client QPS")
	flag.IntVar(&flagVar.clientBurst, "k8s-client-burst", defaultClientBurst, "kubernetes client Burst")
	flag.StringVar(&flagVar.moduleVerificationKeyFilePath, "module-verification-key-file", "",
		"This verification key is used to verify modules against their signature")
	flag.StringVar(&flagVar.moduleVerificationKeyFilePath, "module-verification-signature-names",
		"kyma-module-signature:kyma-extension-signature",
		"This verification key list is used to verify modules against their signature")
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
	flag.StringVar(&flagVar.virtualServiceName, "virtual-svc-name", "kcp-events",
		"Name of the virtual service resource to be reconciled by the watcher control loop.")
	flag.BoolVar(&flagVar.enableWatcherLocalTesting, "enable-watcher-local-testing", false,
		"Enabling KCP Watcher two-cluster setup to be tested locally using k3d")
	flag.IntVar(&flagVar.listenerHTTPPortLocalMapping, "listener-http-local-mapping", defaultListenerPort,
		"Port that is mapped to HTTP port of the local k3d cluster using --port 9080:80@loadbalancer when "+
			"creating the KCP cluster")
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
	return flagVar
}

type FlagVar struct {
	metricsAddr                                                     string
	enableLeaderElection                                            bool
	probeAddr                                                       string
	listenerAddr                                                    string
	maxConcurrentReconciles                                         int
	kymaRequeueSuccessInterval                                      time.Duration
	watcherRequeueSuccessInterval                                   time.Duration
	moduleVerificationKeyFilePath, moduleVerificationSignatureNames string
	clientQPS                                                       float64
	clientBurst                                                     int
	enableWebhooks                                                  bool
	enableKcpWatcher                                                bool
	skrWatcherPath                                                  string
	skrWebhookMemoryLimits                                          string
	skrWebhookCPULimits                                             string
	virtualServiceName                                              string
	enableWatcherLocalTesting                                       bool
	// listenerHTTPPortLocalMapping is used to enable the user
	// to specify the port used to expose the KCP cluster for the watcher
	// when testing locally using dual-k3d cluster-setup
	// (only k3d clusters are supported for watcher local testing)
	listenerHTTPPortLocalMapping           int
	pprof                                  bool
	pprofAddr                              string
	pprofServerTimeout                     time.Duration
	failureBaseDelay, failureMaxDelay      time.Duration
	rateLimiterBurst, rateLimiterFrequency int
	cacheSyncTimeout                       time.Duration
	enableDomainNameVerification           bool
	logLevel                               int
}
