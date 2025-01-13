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
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller/istiogatewaysecret"
	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma"
	"github.com/kyma-project/lifecycle-manager/internal/controller/mandatorymodule"
	"github.com/kyma-project/lifecycle-manager/internal/controller/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/controller/purge"
	watcherctrl "github.com/kyma-project/lifecycle-manager/internal/controller/watcher"
	"github.com/kyma-project/lifecycle-manager/internal/crd"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/maintenancewindows"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/matcher"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	_ "ocm.software/ocm/api/ocm"
	//nolint:gci // kubebuilder's scaffold imports must be appended here.
)

const (
	metricCleanupTimeout    = 5 * time.Minute
	bootstrapFailedExitCode = 1
	runtimeProblemExitCode  = 2
)

var (
	buildVersion                         = "not_provided" //nolint:gochecknoglobals // used to embed static binary version during release builds
	errFailedToDropStoredVersions        = errors.New("failed to drop stored versions")
	errFailedToScheduleMetricsCleanupJob = errors.New("failed to schedule metrics cleanup job")
)

func registerSchemas(scheme *machineryruntime.Scheme) {
	machineryutilruntime.Must(k8sclientscheme.AddToScheme(scheme))
	machineryutilruntime.Must(api.AddToScheme(scheme))
	machineryutilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	machineryutilruntime.Must(certmanagerv1.AddToScheme(scheme))
	machineryutilruntime.Must(istioclientapiv1beta1.AddToScheme(scheme))
	machineryutilruntime.Must(v1beta2.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	setupLog := ctrl.Log.WithName("setup")
	scheme := machineryruntime.NewScheme()
	registerSchemas(scheme)

	flagVar := flags.DefineFlagVar()
	flag.Parse()
	ctrl.SetLogger(log.ConfigLogger(int8(flagVar.LogLevel), //nolint:gosec // loglevel should always be between -128 to 127
		zapcore.Lock(os.Stdout)))
	setupLog.Info("starting Lifecycle-Manager version: " + buildVersion)
	if err := flagVar.Validate(); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(bootstrapFailedExitCode)
	}
	if flagVar.Pprof {
		go pprofStartServer(flagVar.PprofAddr, flagVar.PprofServerTimeout, setupLog)
	}

	cacheOptions := internal.GetCacheOptions(flagVar.IsKymaManaged, flagVar.IstioNamespace,
		flagVar.IstioGatewayNamespace, flagVar.RemoteSyncNamespace)
	setupManager(flagVar, cacheOptions, scheme, setupLog)
}

func pprofStartServer(addr string, timeout time.Duration, setupLog logr.Logger) {
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

//nolint: funlen // setupManager is a main function that sets up the manager
func setupManager(flagVar *flags.FlagVar, cacheOptions cache.Options, scheme *machineryruntime.Scheme,
	setupLog logr.Logger,
) {
	config := ctrl.GetConfigOrDie()
	config.QPS = float32(flagVar.ClientQPS)
	config.Burst = flagVar.ClientBurst

	mgr, err := ctrl.NewManager(
		config, ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: flagVar.MetricsAddr,
			},
			HealthProbeBindAddress: flagVar.ProbeAddr,
			LeaderElection:         flagVar.EnableLeaderElection,
			LeaderElectionID:       "893110f7.kyma-project.io",
			LeaseDuration:          &flagVar.LeaderElectionLeaseDuration,
			RenewDeadline:          &flagVar.LeaderElectionRenewDeadline,
			RetryPeriod:            &flagVar.LeaderElectionRetryPeriod,
			Cache:                  cacheOptions,
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(bootstrapFailedExitCode)
	}
	kcpRestConfig := mgr.GetConfig()
	remoteClientCache := remote.NewClientCache()
	kcpClient := remote.NewClientWithConfig(mgr.GetClient(), kcpRestConfig)
	eventRecorder := event.NewRecorderWrapper(mgr.GetEventRecorderFor(shared.OperatorName))
	skrContextProvider := remote.NewKymaSkrContextProvider(kcpClient, remoteClientCache, eventRecorder)
	var skrWebhookManager *watcher.SKRWebhookManifestManager
	var options ctrlruntime.Options
	if flagVar.EnableKcpWatcher {
		if skrWebhookManager, err = createSkrWebhookManager(mgr, skrContextProvider, flagVar); err != nil {
			setupLog.Error(err, "failed to create skr webhook manager")
			os.Exit(bootstrapFailedExitCode)
		}
		setupKcpWatcherReconciler(mgr, options, eventRecorder, flagVar, setupLog)
		err = istiogatewaysecret.SetupReconciler(mgr, flagVar, options)
		if err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Istio")
			os.Exit(bootstrapFailedExitCode)
		}
	}

	sharedMetrics := metrics.NewSharedMetrics()
	descriptorProvider := provider.NewCachedDescriptorProvider()
	kymaMetrics := metrics.NewKymaMetrics(sharedMetrics)
	mandatoryModulesMetrics := metrics.NewMandatoryModulesMetrics()
	moduleMetrics := metrics.NewModuleMetrics()

	// The maintenance windows policy should be passed to the manifest reconciler to be resolved: https://github.com/kyma-project/lifecycle-manager/issues/2101
	_, err = maintenancewindows.InitializeMaintenanceWindowsPolicy(setupLog)
	if err != nil {
		setupLog.Error(err, "unable to set maintenance windows policy")
	}
	setupKymaReconciler(mgr, descriptorProvider, skrContextProvider, eventRecorder, flagVar, options, skrWebhookManager,
		kymaMetrics, moduleMetrics, setupLog)
	setupManifestReconciler(mgr, flagVar, options, sharedMetrics, mandatoryModulesMetrics, moduleMetrics, setupLog,
		eventRecorder)
	setupMandatoryModuleReconciler(mgr, descriptorProvider, flagVar, options, mandatoryModulesMetrics, setupLog)
	setupMandatoryModuleDeletionReconciler(mgr, descriptorProvider, eventRecorder, flagVar, options, setupLog)
	if flagVar.EnablePurgeFinalizer {
		setupPurgeReconciler(mgr, skrContextProvider, eventRecorder, flagVar, options, setupLog)
	}

	if flagVar.EnableWebhooks {
		// enable conversion webhook for CRDs here

		setupLog.Info("currently no configured webhooks")
	}

	addHealthChecks(mgr, setupLog)

	go cleanupStoredVersions(flagVar.DropCrdStoredVersionMap, mgr, setupLog)
	go scheduleMetricsCleanup(kymaMetrics, flagVar.MetricsCleanupIntervalInMinutes, mgr, setupLog)

	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(runtimeProblemExitCode)
	}
}

func addHealthChecks(mgr manager.Manager, setupLog logr.Logger) {
	// +kubebuilder:scaffold:builder
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
}

func cleanupStoredVersions(crdVersionsToDrop string, mgr manager.Manager, setupLog logr.Logger) {
	if crdVersionsToDrop == "" {
		return
	}

	ctx := context.Background()
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		setupLog.V(log.InfoLevel).Error(errFailedToDropStoredVersions, "failed to sync cache")
		return
	}

	crd.DropStoredVersion(ctx, mgr.GetClient(), crdVersionsToDrop)
}

func scheduleMetricsCleanup(kymaMetrics *metrics.KymaMetrics, cleanupIntervalInMinutes int, mgr manager.Manager,
	setupLog logr.Logger,
) {
	ctx := context.Background()
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		setupLog.V(log.InfoLevel).Error(errFailedToScheduleMetricsCleanupJob, "failed to sync cache")
		return
	}

	scheduler := gocron.NewScheduler(time.UTC)
	_, scheduleErr := scheduler.Every(cleanupIntervalInMinutes).Minutes().Do(func() {
		ctx, cancel := context.WithTimeout(ctx, metricCleanupTimeout)
		defer cancel()
		if err := kymaMetrics.CleanupNonExistingKymaCrsMetrics(ctx, mgr.GetClient()); err != nil {
			setupLog.Info(fmt.Sprintf("failed to cleanup non existing kyma crs metrics, err: %s", err))
		}
	})
	if scheduleErr != nil {
		setupLog.Info(fmt.Sprintf("failed to setup cleanup routine for non existing kyma crs metrics, err: %s",
			scheduleErr))
	}
	scheduler.StartAsync()
	setupLog.V(log.DebugLevel).Info("scheduled job for cleaning up metrics")
}

func setupKymaReconciler(mgr ctrl.Manager, descriptorProvider *provider.CachedDescriptorProvider,
	skrContextFactory remote.SkrContextProvider, event event.Event, flagVar *flags.FlagVar, options ctrlruntime.Options,
	skrWebhookManager *watcher.SKRWebhookManifestManager, kymaMetrics *metrics.KymaMetrics,
	moduleMetrics *metrics.ModuleMetrics, setupLog logr.Logger,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentKymaReconciles

	if err := (&kyma.Reconciler{
		Client:             mgr.GetClient(),
		SkrContextFactory:  skrContextFactory,
		Event:              event,
		DescriptorProvider: descriptorProvider,
		SyncRemoteCrds:     remote.NewSyncCrdsUseCase(mgr.GetClient(), skrContextFactory, nil),
		SKRWebhookManager:  skrWebhookManager,
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.KymaRequeueSuccessInterval,
			Busy:    flagVar.KymaRequeueBusyInterval,
			Error:   flagVar.KymaRequeueErrInterval,
			Warning: flagVar.KymaRequeueWarningInterval,
		},
		InKCPMode:           flagVar.InKCPMode,
		RemoteSyncNamespace: flagVar.RemoteSyncNamespace,
		IsManagedKyma:       flagVar.IsKymaManaged,
		Metrics:             kymaMetrics,
		ModuleMetrics:       moduleMetrics,
		RemoteCatalog: remote.NewRemoteCatalogFromKyma(mgr.GetClient(), skrContextFactory,
			flagVar.RemoteSyncNamespace),
	}).SetupWithManager(
		mgr, options, kyma.SetupOptions{
			ListenerAddr:                 flagVar.KymaListenerAddr,
			EnableDomainNameVerification: flagVar.EnableDomainNameVerification,
			IstioNamespace:               flagVar.IstioNamespace,
		},
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kyma")
		os.Exit(1)
	}
}

func createSkrWebhookManager(mgr ctrl.Manager, skrContextFactory remote.SkrContextProvider,
	flagVar *flags.FlagVar,
) (*watcher.SKRWebhookManifestManager, error) {
	config := watcher.SkrWebhookManagerConfig{
		SKRWatcherPath:         flagVar.WatcherResourcesPath,
		SkrWatcherImage:        flagVar.GetWatcherImage(),
		SkrWebhookCPULimits:    flagVar.WatcherResourceLimitsCPU,
		SkrWebhookMemoryLimits: flagVar.WatcherResourceLimitsMemory,
		RemoteSyncNamespace:    flagVar.RemoteSyncNamespace,
	}
	certConfig := watcher.CertificateConfig{
		IstioNamespace:      flagVar.IstioNamespace,
		RemoteSyncNamespace: flagVar.RemoteSyncNamespace,
		CACertificateName:   flagVar.CaCertName,
		AdditionalDNSNames:  strings.Split(flagVar.AdditionalDNSNames, ","),
		Duration:            flagVar.SelfSignedCertDuration,
		RenewBefore:         flagVar.SelfSignedCertRenewBefore,
		KeySize:             flagVar.SelfSignedCertKeySize,
	}
	gatewayConfig := watcher.GatewayConfig{
		IstioGatewayName:          flagVar.IstioGatewayName,
		IstioGatewayNamespace:     flagVar.IstioGatewayNamespace,
		LocalGatewayPortOverwrite: flagVar.ListenerPortOverwrite,
	}

	resolvedKcpAddr, err := gatewayConfig.ResolveKcpAddr(mgr)
	if err != nil {
		return nil, err
	}
	return watcher.NewSKRWebhookManifestManager(
		mgr.GetClient(),
		skrContextFactory,
		config,
		certConfig,
		resolvedKcpAddr)
}

func setupPurgeReconciler(mgr ctrl.Manager,
	skrContextProvider remote.SkrContextProvider,
	event event.Event,
	flagVar *flags.FlagVar,
	options ctrlruntime.Options,
	setupLog logr.Logger,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout

	if err := (&purge.Reconciler{
		Client:                mgr.GetClient(),
		SkrContextFactory:     skrContextProvider,
		Event:                 event,
		PurgeFinalizerTimeout: flagVar.PurgeFinalizerTimeout,
		SkipCRDs:              matcher.CreateCRDMatcherFrom(flagVar.SkipPurgingFor),
		IsManagedKyma:         flagVar.IsKymaManaged,
		Metrics:               metrics.NewPurgeMetrics(),
	}).SetupWithManager(
		mgr, options,
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PurgeReconciler")
		os.Exit(bootstrapFailedExitCode)
	}
}

func setupManifestReconciler(mgr ctrl.Manager, flagVar *flags.FlagVar, options ctrlruntime.Options,
	sharedMetrics *metrics.SharedMetrics, mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	moduleMetrics *metrics.ModuleMetrics, setupLog logr.Logger, event event.Event,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentManifestReconciles

	manifestClient := manifestclient.NewManifestClient(event, mgr.GetClient())

	if err := manifest.SetupWithManager(
		mgr, options, queue.RequeueIntervals{
			Success: flagVar.ManifestRequeueSuccessInterval,
			Busy:    flagVar.ManifestRequeueBusyInterval,
			Error:   flagVar.ManifestRequeueErrInterval,
			Warning: flagVar.ManifestRequeueWarningInterval,
			Jitter: queue.NewRequeueJitter(flagVar.ManifestRequeueJitterProbability,
				flagVar.ManifestRequeueJitterPercentage),
		}, manifest.SetupOptions{
			ListenerAddr:                 flagVar.ManifestListenerAddr,
			EnableDomainNameVerification: flagVar.EnableDomainNameVerification,
		}, metrics.NewManifestMetrics(sharedMetrics), mandatoryModulesMetrics, moduleMetrics,
		manifestClient,
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Manifest")
		os.Exit(bootstrapFailedExitCode)
	}
}

func setupKcpWatcherReconciler(mgr ctrl.Manager, options ctrlruntime.Options, event event.Event, flagVar *flags.FlagVar,
	setupLog logr.Logger,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentWatcherReconciles

	if err := (&watcherctrl.Reconciler{
		Client:     mgr.GetClient(),
		Event:      event,
		Scheme:     mgr.GetScheme(),
		RestConfig: mgr.GetConfig(),
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.WatcherRequeueSuccessInterval,
			Busy:    flags.DefaultKymaRequeueBusyInterval,
			Error:   flags.DefaultKymaRequeueErrInterval,
			Warning: flags.DefaultKymaRequeueWarningInterval,
		},
		IstioGatewayNamespace: flagVar.IstioGatewayNamespace,
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create watcher controller")
		os.Exit(bootstrapFailedExitCode)
	}
}

func setupMandatoryModuleReconciler(mgr ctrl.Manager,
	descriptorProvider *provider.CachedDescriptorProvider,
	flagVar *flags.FlagVar,
	options ctrlruntime.Options,
	metrics *metrics.MandatoryModulesMetrics,
	setupLog logr.Logger,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentMandatoryModuleReconciles

	if err := (&mandatorymodule.InstallationReconciler{
		Client: mgr.GetClient(),
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.MandatoryModuleRequeueSuccessInterval,
			Busy:    flagVar.KymaRequeueBusyInterval,
			Error:   flagVar.KymaRequeueErrInterval,
			Warning: flagVar.KymaRequeueWarningInterval,
		},
		RemoteSyncNamespace: flagVar.RemoteSyncNamespace,
		InKCPMode:           flagVar.InKCPMode,
		DescriptorProvider:  descriptorProvider,
		Metrics:             metrics,
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MandatoryModule")
		os.Exit(bootstrapFailedExitCode)
	}
}

func setupMandatoryModuleDeletionReconciler(mgr ctrl.Manager,
	descriptorProvider *provider.CachedDescriptorProvider,
	event event.Event,
	flagVar *flags.FlagVar,
	options ctrlruntime.Options,
	setupLog logr.Logger,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentMandatoryModuleDeletionReconciles

	if err := (&mandatorymodule.DeletionReconciler{
		Client:             mgr.GetClient(),
		Event:              event,
		DescriptorProvider: descriptorProvider,
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.MandatoryModuleDeletionRequeueSuccessInterval,
			Busy:    flagVar.KymaRequeueBusyInterval,
			Error:   flagVar.KymaRequeueErrInterval,
			Warning: flagVar.KymaRequeueWarningInterval,
		},
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MandatoryModule")
		os.Exit(bootstrapFailedExitCode)
	}
}
