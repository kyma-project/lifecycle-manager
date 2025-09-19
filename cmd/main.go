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
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/cmd/composition/service/skrwebhook"
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
	gatewaysecretclient "github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/client"
	"github.com/kyma-project/lifecycle-manager/internal/maintenancewindows"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/keychainprovider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/repository/istiogateway"
	kymarepository "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	secretrepository "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules/generator"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules/generator/fromerror"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
	skrclientcache "github.com/kyma-project/lifecycle-manager/internal/service/skrclient/cache"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/matcher"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	_ "ocm.software/ocm/api/ocm"
)

const (
	metricCleanupTimeout    = 5 * time.Minute
	bootstrapFailedExitCode = 1
	runtimeProblemExitCode  = 2

	maintenanceWindowPolicyName        = "policy"
	maintenanceWindowPoliciesDirectory = "/etc/maintenance-policy"
)

var (
	buildVersion                         = "not_provided" //nolint:gochecknoglobals,revive // used to embed static binary version during release builds
	errFailedToDropStoredVersions        = errors.New("failed to drop stored versions")
	errFailedToScheduleMetricsCleanupJob = errors.New("failed to schedule metrics cleanup job")
)

func registerSchemas(scheme *machineryruntime.Scheme) {
	machineryutilruntime.Must(k8sclientscheme.AddToScheme(scheme))
	machineryutilruntime.Must(api.AddToScheme(scheme))
	machineryutilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	machineryutilruntime.Must(certmanagerv1.AddToScheme(scheme))
	machineryutilruntime.Must(gcertv1alpha1.AddToScheme(scheme))
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
	ctrl.SetLogger(
		log.ConfigLogger(int8(flagVar.LogLevel), //nolint:gosec // loglevel should always be between -128 to 127
			zapcore.Lock(os.Stdout)),
	)
	setupLog.Info("starting Lifecycle-Manager version: " + buildVersion)
	if err := flagVar.Validate(); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(bootstrapFailedExitCode)
	}
	if flagVar.Pprof {
		go pprofStartServer(flagVar.PprofAddr, flagVar.PprofServerTimeout, setupLog)
	}

	cacheOptions := setup.SetupCacheOptions(flagVar.IsKymaManaged,
		flagVar.IstioNamespace,
		flagVar.IstioGatewayNamespace,
		flagVar.CertificateManagement,
		setupLog,
	)
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

//nolint:funlen // disable length check since the function is the composition root
func setupManager(flagVar *flags.FlagVar, cacheOptions cache.Options, scheme *machineryruntime.Scheme,
	logger logr.Logger,
) {
	mgr, err := configManager(flagVar, cacheOptions, scheme)
	if err != nil {
		logger.Error(err, "unable to start manager")
		os.Exit(bootstrapFailedExitCode)
	}
	kcpRestConfig := mgr.GetConfig()
	remoteClientCache := remote.NewClientCache()
	kcpClient := remote.NewClientWithConfig(mgr.GetClient(), kcpRestConfig)
	eventRecorder := event.NewRecorderWrapper(mgr.GetEventRecorderFor(shared.OperatorName))

	kcpClientWithoutCache, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		logger.Error(err, "can't create kcpClient")
		os.Exit(bootstrapFailedExitCode)
	}
	gatewayRepository := istiogateway.NewRepository(kcpClientWithoutCache)
	accessSecretRepository := secretrepository.NewRepository(kcpClientWithoutCache, shared.DefaultControlPlaneNamespace)
	accessManagerService := accessmanager.NewService(accessSecretRepository)
	skrContextProvider := remote.NewKymaSkrContextProvider(kcpClient, remoteClientCache, eventRecorder,
		accessManagerService)
	var skrWebhookManager *watcher.SkrWebhookManifestManager
	var options ctrlruntime.Options
	if flagVar.EnableKcpWatcher {
		skrWebhookManager, err = skrwebhook.ComposeSkrWebhookManager(kcpClient, skrContextProvider,
			gatewayRepository,
			flagVar)
		if err != nil {
			logger.Error(err, "failed to setup SKR webhook manager")
			os.Exit(bootstrapFailedExitCode)
		}
		setupKcpWatcherReconciler(mgr, options, eventRecorder, flagVar, logger)
		var gatewaysecretclnt gatewaysecretclient.CertificateInterface
		gatewaysecretclnt, err = setup.SetupCertInterface(kcpClient, flagVar)
		if err != nil {
			logger.Error(err, "failed to setup certificate client")
			os.Exit(bootstrapFailedExitCode)
		}
		err = istiogatewaysecret.SetupReconciler(mgr, gatewaysecretclnt,
			flagVar,
			options)
		if err != nil {
			logger.Error(err, "unable to create controller", "controller", "Istio")
			os.Exit(bootstrapFailedExitCode)
		}
	}

	sharedMetrics := metrics.NewSharedMetrics()
	descriptorProvider := provider.NewCachedDescriptorProvider()
	kymaMetrics := metrics.NewKymaMetrics(sharedMetrics)
	mandatoryModulesMetrics := metrics.NewMandatoryModulesMetrics()
	maintenanceWindow := initMaintenanceWindow(flagVar.MinMaintenanceWindowSize, logger)
	metrics.NewFipsMetrics().Update()

	//nolint:godox // this will be used in the future
	// TODO: use the oci registry host //nolint:godox // this will be used in the future
	_ = getOciRegistryHost(mgr.GetConfig(), flagVar, logger)

	setupKymaReconciler(mgr, descriptorProvider, skrContextProvider, eventRecorder, flagVar, options, skrWebhookManager,
		kymaMetrics, logger, maintenanceWindow)
	setupManifestReconciler(mgr, flagVar, options, sharedMetrics, mandatoryModulesMetrics, accessManagerService, logger,
		eventRecorder)
	setupMandatoryModuleReconciler(mgr, descriptorProvider, flagVar, options, mandatoryModulesMetrics, logger)
	setupMandatoryModuleDeletionReconciler(mgr, descriptorProvider, eventRecorder, flagVar, options, logger)
	if flagVar.EnablePurgeFinalizer {
		setupPurgeReconciler(mgr, skrContextProvider, eventRecorder, flagVar, options, logger)
	}

	if flagVar.EnableWebhooks {
		// enable conversion webhook for CRDs here

		logger.Info("currently no configured webhooks")
	}

	addHealthChecks(mgr, logger)

	go cleanupStoredVersions(flagVar.DropCrdStoredVersionMap, mgr, logger)
	go scheduleMetricsCleanup(kymaMetrics, flagVar.MetricsCleanupIntervalInMinutes, mgr, logger)

	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "problem running manager")
		os.Exit(runtimeProblemExitCode)
	}
}

func getOciRegistryHost(config *rest.Config, flagVar *flags.FlagVar, setupLog logr.Logger) string {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes clientset")
		os.Exit(bootstrapFailedExitCode)
	}
	secretInterface := clientset.CoreV1().Secrets(shared.DefaultControlPlaneNamespace)

	ociRegistrySetup, err := setup.NewOCIRegistryHostProvider(secretInterface, flagVar.OciRegistryHost,
		flagVar.OciRegistryCredSecretName)
	if err != nil {
		setupLog.Error(err, "failed to setup OCI registry")
		os.Exit(bootstrapFailedExitCode)
	}
	ociRegistryHost, err := ociRegistrySetup.ResolveHost(context.Background())
	if err != nil {
		setupLog.Error(err, "failed to resolve OCI registry host")
		os.Exit(bootstrapFailedExitCode)
	}
	return ociRegistryHost
}

func initMaintenanceWindow(minWindowSize time.Duration, logger logr.Logger) maintenancewindows.MaintenanceWindow {
	maintenanceWindowsMetrics := metrics.NewMaintenanceWindowMetrics()
	maintenanceWindow, err := maintenancewindows.InitializeMaintenanceWindow(logger,
		maintenanceWindowPoliciesDirectory,
		maintenanceWindowPolicyName,
		minWindowSize)
	if err != nil {
		maintenanceWindowsMetrics.RecordConfigReadSuccess(false)
		logger.Error(err, "unable to set maintenance windows policy")
	} else {
		maintenanceWindowsMetrics.RecordConfigReadSuccess(true)
	}
	return maintenanceWindow
}

//nolint:ireturn // the implementation is not a part of the public API
func configManager(flagVar *flags.FlagVar, cacheOptions cache.Options,
	scheme *machineryruntime.Scheme,
) (manager.Manager, error) {
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
		return nil, fmt.Errorf("unable to create manager: %w", err)
	}
	return mgr, nil
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
	skrWebhookManager *watcher.SkrWebhookManifestManager, kymaMetrics *metrics.KymaMetrics,
	setupLog logr.Logger, maintenanceWindow maintenancewindows.MaintenanceWindow,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentKymaReconciles

	moduleTemplateInfoLookupStrategies := moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
		[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
			moduletemplateinfolookup.NewByVersionStrategy(mgr.GetClient()),
			moduletemplateinfolookup.NewByChannelStrategy(mgr.GetClient()),
			moduletemplateinfolookup.NewWithMaintenanceWindowDecorator(maintenanceWindow,
				moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(mgr.GetClient())),
		},
	)

	kcpClient := mgr.GetClient()
	moduleStatusGen := generator.NewModuleStatusGenerator(fromerror.GenerateModuleStatusFromError)
	modulesStatusHandler := modules.NewStatusHandler(moduleStatusGen, kcpClient, kymaMetrics.RemoveModuleStateMetrics)

	if err := (&kyma.Reconciler{
		Client:               kcpClient,
		SkrContextFactory:    skrContextFactory,
		Event:                event,
		DescriptorProvider:   descriptorProvider,
		SyncRemoteCrds:       remote.NewSyncCrdsUseCase(kcpClient, skrContextFactory, nil),
		ModulesStatusHandler: modulesStatusHandler,
		SKRWebhookManager:    skrWebhookManager,
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.KymaRequeueSuccessInterval,
			Busy:    flagVar.KymaRequeueBusyInterval,
			Error:   flagVar.KymaRequeueErrInterval,
			Warning: flagVar.KymaRequeueWarningInterval,
		},
		RemoteSyncNamespace: flagVar.RemoteSyncNamespace,
		IsManagedKyma:       flagVar.IsKymaManaged,
		Metrics:             kymaMetrics,
		RemoteCatalog: remote.NewRemoteCatalogFromKyma(kcpClient, skrContextFactory,
			flagVar.RemoteSyncNamespace),
		TemplateLookup: templatelookup.NewTemplateLookup(kcpClient, descriptorProvider,
			moduleTemplateInfoLookupStrategies),
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

func setupManifestReconciler(mgr ctrl.Manager,
	flagVar *flags.FlagVar,
	options ctrlruntime.Options,
	sharedMetrics *metrics.SharedMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	accessManagerService *accessmanager.Service,
	setupLog logr.Logger,
	event event.Event,
) {
	options.RateLimiter = internal.RateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)
	options.CacheSyncTimeout = flagVar.CacheSyncTimeout
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentManifestReconciles

	manifestClient := manifestclient.NewManifestClient(event, mgr.GetClient())
	orphanDetectionClient := kymarepository.NewClient(mgr.GetClient())
	specResolver := spec.NewResolver(keychainLookupFromFlag(mgr, flagVar), img.NewPathExtractor())
	clientCache := skrclientcache.NewService()
	skrClient := skrclient.NewService(mgr.GetConfig().QPS, mgr.GetConfig().Burst, accessManagerService)
	if err := manifest.SetupWithManager(mgr, options, queue.RequeueIntervals{
		Success: flagVar.ManifestRequeueSuccessInterval,
		Busy:    flagVar.ManifestRequeueBusyInterval,
		Error:   flagVar.ManifestRequeueErrInterval,
		Warning: flagVar.ManifestRequeueWarningInterval,
		Jitter: queue.NewRequeueJitter(flagVar.ManifestRequeueJitterProbability,
			flagVar.ManifestRequeueJitterPercentage),
	}, manifest.SetupOptions{
		ListenerAddr:                 flagVar.ManifestListenerAddr,
		EnableDomainNameVerification: flagVar.EnableDomainNameVerification,
	}, metrics.NewManifestMetrics(sharedMetrics), mandatoryModulesMetrics, manifestClient, orphanDetectionClient,
		specResolver, clientCache, skrClient); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Manifest")
		os.Exit(bootstrapFailedExitCode)
	}
}

//nolint:ireturn // constructor functions can return interfaces
func keychainLookupFromFlag(mgr ctrl.Manager, flagVar *flags.FlagVar) spec.KeyChainLookup {
	if flagVar.OciRegistryCredSecretName != "" {
		return keychainprovider.NewFromSecretKeyChainProvider(mgr.GetClient(),
			types.NamespacedName{
				Namespace: shared.DefaultControlPlaneNamespace,
				Name:      flagVar.OciRegistryCredSecretName,
			})
	}
	return keychainprovider.NewDefaultKeyChainProvider()
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
