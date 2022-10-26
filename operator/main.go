/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/signature"

	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/controllers"
	moduleManagerV1alpha1 "github.com/kyma-project/module-manager/operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

const (
	failureBaseDelayDefault       = 100 * time.Millisecond
	failureMaxDelayDefault        = 1000 * time.Second
	rateLimiterFrequencyDefault   = 30
	rateLimiterBurstDefault       = 200
	port                          = 9443
	defaultRequeueSuccessInterval = 20 * time.Second
	defaultRequeueFailureInterval = 10 * time.Second
	defaultRequeueWaitingInterval = 3 * time.Second
	defaultClientQPS              = 150
	defaultClientBurst            = 150
	defaultPprofServerTimeout     = 90 * time.Second
)

var (
	scheme   = runtime.NewScheme()        //nolint:gochecknoglobals
	setupLog = ctrl.Log.WithName("setup") //nolint:gochecknoglobals
)

//nolint:gochecknoinits
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(v1extensions.AddToScheme(scheme))
	utilruntime.Must(moduleManagerV1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type FlagVar struct {
	metricsAddr                                                            string
	enableLeaderElection                                                   bool
	probeAddr                                                              string
	listenerAddr                                                           string
	maxConcurrentReconciles                                                int
	requeueSuccessInterval, requeueFailureInterval, requeueWaitingInterval time.Duration
	moduleVerificationKeyFilePath, moduleVerificationSignatureNames        string
	clientQPS                                                              float64
	clientBurst                                                            int
	enableWebhooks                                                         bool
	enableKcpWatcher                                                       bool
	skrWatcherPath                                                         string
	skrWebhookMemoryLimits                                                 string
	skrWebhookCPULimits                                                    string
	pprof                                                                  bool
	pprofAddr                                                              string
	pprofServerTimeout                                                     time.Duration
	failureBaseDelay, failureMaxDelay                                      time.Duration
	rateLimiterBurst, rateLimiterFrequency                                 int
}

func main() {
	flagVar := defineFlagVar()

	opts := zap.Options{
		Development: true,
		Level:       zapcore.DebugLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if flagVar.pprof {
		go pprofStartServer(flagVar.pprofAddr, flagVar.pprofServerTimeout)
	}

	setupManager(flagVar, controllers.NewCacheFunc(), scheme)
}

func pprofStartServer(addr string, timeout time.Duration) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       timeout,
		ReadHeaderTimeout: timeout,
		WriteTimeout:      timeout,
	}

	if err := server.ListenAndServe(); err != nil {
		setupLog.Error(err, "error starting pprof server")
	}
}

func setupManager(flagVar *FlagVar, newCacheFunc cache.NewCacheFunc, scheme *runtime.Scheme) {
	config := ctrl.GetConfigOrDie()
	config.QPS = float32(flagVar.clientQPS)
	config.Burst = flagVar.clientBurst

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     flagVar.metricsAddr,
		Port:                   port,
		HealthProbeBindAddress: flagVar.probeAddr,
		LeaderElection:         flagVar.enableLeaderElection,
		LeaderElectionID:       "893110f7.kyma-project.io",
		NewCache:               newCacheFunc,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	intervals := controllers.RequeueIntervals{
		Success: flagVar.requeueSuccessInterval,
		Failure: flagVar.requeueFailureInterval,
		Waiting: flagVar.requeueWaitingInterval,
	}
	options := controller.Options{
		RateLimiter: workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(flagVar.failureBaseDelay, flagVar.failureMaxDelay),
			&workqueue.BucketRateLimiter{
				Limiter: rate.NewLimiter(rate.Limit(flagVar.rateLimiterFrequency), flagVar.rateLimiterBurst),
			}),
		MaxConcurrentReconciles: flagVar.maxConcurrentReconciles,
	}

	remoteClientCache := remote.NewClientCache()

	setupKymaReconciler(mgr, remoteClientCache, flagVar, intervals, options)

	if flagVar.enableKcpWatcher {
		setupKcpWatcherReconciler(mgr, flagVar, intervals, options)
	}
	if flagVar.enableWebhooks {
		if err := (&operatorv1alpha1.ModuleTemplate{}).
			SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "ModuleTemplate")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

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
	flag.DurationVar(&flagVar.requeueFailureInterval, "requeue-failure-interval", defaultRequeueFailureInterval,
		"determines the duration after which a failing reconciliation is retried and "+
			"enqueued for a next try at recovering (e.g. because an Remote Synchronization Interaction failed)")
	flag.DurationVar(&flagVar.requeueWaitingInterval, "requeue-waiting-interval", defaultRequeueWaitingInterval,
		"etermines the duration after which a pending reconciliation is requeued "+
			"if the operator decides that it needs to wait for a certain state to update before it can proceed "+
			"(e.g. because of pending finalizers in the deletion process)")
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
	flag.StringVar(&flagVar.skrWatcherPath, "skr-watcher-path", "skr-webhook",
		"The path to the skr watcher chart.")
	flag.StringVar(&flagVar.skrWebhookMemoryLimits, "skr-webhook-memory-limits", "200Mi",
		"The resources.limits.memory for skr webhook.")
	flag.StringVar(&flagVar.skrWebhookCPULimits, "skr-webhook-cpu-limits", "0.1",
		"The resources.limits.cpu for skr webhook.")
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
	return flagVar
}

func setupKymaReconciler(
	mgr ctrl.Manager,
	remoteClientCache *remote.ClientCache,
	flagVar *FlagVar,
	intervals controllers.RequeueIntervals,
	options controller.Options,
) {
	if err := (&controllers.KymaReconciler{
		Client:            mgr.GetClient(),
		EventRecorder:     mgr.GetEventRecorderFor(operatorv1alpha1.OperatorName),
		RemoteClientCache: remoteClientCache,
		RequeueIntervals:  intervals,
		VerificationSettings: signature.VerificationSettings{
			PublicKeyFilePath:   flagVar.moduleVerificationKeyFilePath,
			ValidSignatureNames: strings.Split(flagVar.moduleVerificationSignatureNames, ":"),
		},
	}).SetupWithManager(mgr, options, flagVar.listenerAddr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kyma")
		os.Exit(1)
	}
}

func setupKcpWatcherReconciler(
	mgr ctrl.Manager,
	flagVar *FlagVar,
	intervals controllers.RequeueIntervals,
	options controller.Options,
) {
	fileInfo, err := os.Stat(flagVar.skrWatcherPath)
	if err != nil || !fileInfo.IsDir() {
		setupLog.Error(err, "failed to read local skr chart")
	}
	watcherConfig := &controllers.WatcherConfig{
		WebhookChartPath:       flagVar.skrWatcherPath,
		SkrWebhookMemoryLimits: flagVar.skrWebhookMemoryLimits,
		SkrWebhookCPULimits:    flagVar.skrWebhookCPULimits,
	}
	if err := (&controllers.WatcherReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		RestConfig:       mgr.GetConfig(),
		RequeueIntervals: intervals,
		Config:           watcherConfig,
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Watcher")
		os.Exit(1)
	}
}
