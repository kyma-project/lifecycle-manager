package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	operatorv1alpha1 "github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	defaultRequeueSuccessInterval = 20 * time.Second
	defaultClientQPS              = 150
	defaultClientBurst            = 150
	defaultPprofServerTimeout     = 90 * time.Second
	rateLimiterBurstDefault       = 200
	rateLimiterFrequencyDefault   = 30
	failureBaseDelayDefault       = 100 * time.Millisecond
	failureMaxDelayDefault        = 1000 * time.Second
	defaultCacheSyncTimeout       = 2 * time.Minute
	namespacedNamePartsCnt        = 2
	gatewayLabelSelMinLen         = 1
	gatewayLabelSelMaxLen         = 512
)

var (
	errInvalidGatewayFmt       = errors.New("must be <namespace>/<name>")
	errInvalidGatewayNamespace = errors.New("must be an RFC 1123 DNS Label")
	errInvalidGatewayName      = errors.New("must be an RFC 1035 Label Name")
	errTooLong                 = errors.New("value is too long")
	errTooShort                = errors.New("value is too short")
)

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
	flag.DurationVar(&flagVar.requeueSuccessInterval, "requeue-success-interval", defaultRequeueSuccessInterval,
		"determines the duration after which an already successfully reconciled Kyma is enqueued for checking "+
			"if it's still in a consistent state.")
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
	flag.StringVar(&flagVar.skrWatcherPath, "skr-watcher-path", "charts/skr-webhook",
		"The path to the skr watcher chart.")
	flag.StringVar(&flagVar.skrWebhookMemoryLimits, "skr-webhook-memory-limits", "200Mi",
		"The resources.limits.memory for skr webhook.")
	flag.StringVar(&flagVar.skrWebhookCPULimits, "skr-webhook-cpu-limits", "0.1",
		"The resources.limits.cpu for skr webhook.")
	flag.StringVar(&flagVar.virtualServiceName, "virtual-svc-name", "kcp-events",
		"Name of the virtual service resource to be reconciled by the watcher control loop.")
	flag.Var(newNamespacedNameVar(&flagVar.gatewayNamespacedName), "gateway-ns-name",
		"Namespaced name of the gateway resource that the virtual service will use. "+
			"Format: <namespace>/<name>. Example: \"my-namespace/my-gateway\".")
	flag.Var(newStringLenVar(&flagVar.gatewaySelector).
		withMin(gatewayLabelSelMinLen).withMax(gatewayLabelSelMaxLen).
		withDefault(operatorv1alpha1.DefaultIstioGatewaySelector),
		"gateway-selector", "Label selector of the gateway resource that the virtual service will use. "+
			"Ignored if gateway-ns-name flag is specified. Example: \"label1=value1,label2=value2\"")
	flag.BoolVar(&flagVar.pprof, "pprof", false,
		"Whether to start up a pprof server.")
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
	return flagVar
}

type FlagVar struct {
	metricsAddr                                                     string
	enableLeaderElection                                            bool
	probeAddr                                                       string
	listenerAddr                                                    string
	maxConcurrentReconciles                                         int
	requeueSuccessInterval                                          time.Duration
	moduleVerificationKeyFilePath, moduleVerificationSignatureNames string
	clientQPS                                                       float64
	clientBurst                                                     int
	enableWebhooks                                                  bool
	enableKcpWatcher                                                bool
	skrWatcherPath                                                  string
	skrWebhookMemoryLimits                                          string
	skrWebhookCPULimits                                             string
	virtualServiceName                                              string
	gatewayNamespacedName                                           string
	gatewaySelector                                                 string
	pprof                                                           bool
	pprofAddr                                                       string
	pprofServerTimeout                                              time.Duration
	failureBaseDelay, failureMaxDelay                               time.Duration
	rateLimiterBurst, rateLimiterFrequency                          int
	cacheSyncTimeout                                                time.Duration
}

func newNamespacedNameVar(target *string) *namespacedNameVar {
	if target == nil {
		panic("target is nil")
	}

	return &namespacedNameVar{
		target: target,
	}
}

type namespacedNameVar struct {
	target *string
}

func (nnv *namespacedNameVar) String() string {
	/*
		if nnv.target == nil {
			return ""
		}
	*/

	return *nnv.target
}

func (nnv *namespacedNameVar) Set(str string) error {
	parts := strings.Split(str, "/")
	if len(parts) != namespacedNamePartsCnt {
		return fmt.Errorf("invalid format, %w", errInvalidGatewayFmt)
	}

	msgs := utilvalidation.IsDNS1123Label(parts[0])
	if len(msgs) > 0 {
		return fmt.Errorf("%q is invalid, %w", parts[0], errInvalidGatewayNamespace)
	}

	msgs = utilvalidation.IsDNS1035Label(parts[1])
	if len(msgs) > 0 {
		return fmt.Errorf("%q is invalid, %w", parts[1], errInvalidGatewayName)
	}

	*nnv.target = str
	return nil
}

func newStringLenVar(target *string) *stringLenVar {
	if target == nil {
		panic("target is nil")
	}

	return &stringLenVar{
		target: target,
	}
}

type stringLenVar struct {
	minLen uint
	maxLen uint
	target *string
}

func (mlsv *stringLenVar) withMin(min uint) *stringLenVar {
	mlsv.minLen = min
	return mlsv
}

func (mlsv *stringLenVar) withMax(max uint) *stringLenVar {
	mlsv.maxLen = max
	return mlsv
}

func (mlsv *stringLenVar) withDefault(str string) *stringLenVar {
	*mlsv.target = str
	return mlsv
}

func (mlsv *stringLenVar) String() string {
	return *mlsv.target
}

func (mlsv *stringLenVar) Set(str string) error {
	if len(str) < int(mlsv.minLen) {
		return fmt.Errorf("%w, min length is %d", errTooShort, mlsv.minLen)
	}

	if len(str) > int(mlsv.maxLen) {
		return fmt.Errorf("%w, max length is %d", errTooLong, mlsv.maxLen)
	}

	*mlsv.target = str
	return nil
}
