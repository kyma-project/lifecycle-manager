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

	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/deploy"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"

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

	operatorv1alpha1 "github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/controllers"
	moduleManagerV1alpha1 "github.com/kyma-project/module-manager/api/v1alpha1"

	//+kubebuilder:scaffold:imports
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

const (
	port = 9443
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

func main() {
	flagVar := defineFlagVar()
	flag.Parse()
	ctrl.SetLogger(configLogger())

	if flagVar.pprof {
		go pprofStartServer(flagVar.pprofAddr, flagVar.pprofServerTimeout)
	}

	setupManager(flagVar, controllers.NewCacheFunc(), scheme)
}

func configLogger() logr.Logger {
	// The following settings is based on kyma community Improvement of log messages usability
	//nolint:lll
	// https://github.com/kyma-project/community/blob/main/concepts/observability-consistent-logging/improvement-of-log-messages-usability.md#log-structure
	atomicLevel := zap.NewAtomicLevel()
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "date"
	encoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.Lock(os.Stdout), atomicLevel)
	zapLog := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	logger := zapr.NewLogger(zapLog.With(zap.Namespace("context")))
	return logger
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
		NewClient:              NewClient,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	intervals := controllers.RequeueIntervals{
		Success: flagVar.requeueSuccessInterval,
	}
	options := controllerOptionsFromFlagVar(flagVar)

	remoteClientCache := remote.NewClientCache()

	setupKymaReconciler(mgr, remoteClientCache, flagVar, intervals, options)

	if flagVar.enableKcpWatcher {
		setupKcpWatcherReconciler(mgr, intervals, options, flagVar)
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

func controllerOptionsFromFlagVar(flagVar *FlagVar) controller.Options {
	return controller.Options{
		RateLimiter: workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(flagVar.failureBaseDelay, flagVar.failureMaxDelay),
			&workqueue.BucketRateLimiter{
				Limiter: rate.NewLimiter(rate.Limit(flagVar.rateLimiterFrequency), flagVar.rateLimiterBurst),
			},
		),
		MaxConcurrentReconciles: flagVar.maxConcurrentReconciles,
		CacheSyncTimeout:        flagVar.cacheSyncTimeout,
	}
}

func NewClient(
	cache cache.Cache,
	config *rest.Config,
	options client.Options,
	uncachedObjects ...client.Object,
) (client.Client, error) {
	clnt, err := client.New(config, options)
	if err != nil {
		return nil, err
	}
	return client.NewDelegatingClient(
		client.NewDelegatingClientInput{
			CacheReader:     cache,
			Client:          clnt,
			UncachedObjects: uncachedObjects,
		},
	)
}

func setupKymaReconciler(
	mgr ctrl.Manager,
	remoteClientCache *remote.ClientCache,
	flagVar *FlagVar,
	intervals controllers.RequeueIntervals,
	options controller.Options,
) {
	if flagVar.enableKcpWatcher {
		watcherChartDirInfo, err := os.Stat(flagVar.skrWatcherPath)
		if err != nil || !watcherChartDirInfo.IsDir() {
			setupLog.Error(err, "failed to read local skr chart")
		}
	}
	skrChartConfig := &deploy.SkrChartConfig{
		WebhookChartPath:       flagVar.skrWatcherPath,
		SkrWebhookMemoryLimits: flagVar.skrWebhookMemoryLimits,
		SkrWebhookCPULimits:    flagVar.skrWebhookCPULimits,
	}
	skrWebhookChartManager, err := deploy.ResolveSKRWebhookChartManager(flagVar.enableKcpWatcher, skrChartConfig)
	if err != nil {
		setupLog.Error(err, "failed to resolve SKR chart manager")
	}
	if err := (&controllers.KymaReconciler{
		Client:                 mgr.GetClient(),
		EventRecorder:          mgr.GetEventRecorderFor(operatorv1alpha1.OperatorName),
		KcpRestConfig:          mgr.GetConfig(),
		RemoteClientCache:      remoteClientCache,
		SKRWebhookChartManager: skrWebhookChartManager,
		RequeueIntervals:       intervals,
		VerificationSettings: signature.VerificationSettings{
			PublicKeyFilePath:   flagVar.moduleVerificationKeyFilePath,
			ValidSignatureNames: strings.Split(flagVar.moduleVerificationSignatureNames, ":"),
		},
	}).SetupWithManager(mgr, options, controllers.SetupUpSetting{
		ListenerAddr:                 flagVar.listenerAddr,
		EnableDomainNameVerification: flagVar.enableDomainNameVerification,
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kyma")
		os.Exit(1)
	}
}

func setupKcpWatcherReconciler(mgr ctrl.Manager, intervals controllers.RequeueIntervals, options controller.Options,
	flagVar *FlagVar,
) {
	// Set MaxConcurrentReconciles to 1 to avoid concurrent writes on
	// the Istio virtual service resource the WatcherReconciler is managing.
	// In total, we probably only have 20 watcher CRs, one worker can sufficiently handle it,
	// and we don't have to deal with concurrent write to virtual service.
	// although eventually the write operation will succeed.
	options.MaxConcurrentReconciles = 1

	istioConfig := istio.NewConfig(flagVar.virtualServiceName, flagVar.gatewayNamespacedName, &flagVar.gatewaySelector)

	if err := (&controllers.WatcherReconciler{
		Client:           mgr.GetClient(),
		EventRecorder:    mgr.GetEventRecorderFor(controllers.WatcherControllerName),
		Scheme:           mgr.GetScheme(),
		RestConfig:       mgr.GetConfig(),
		RequeueIntervals: intervals,
	}).SetupWithManager(mgr, options, istioConfig); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", controllers.WatcherControllerName)
		os.Exit(1)
	}
}
