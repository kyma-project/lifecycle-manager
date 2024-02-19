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
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-co-op/gocron"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/strings/slices"
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
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/matcher"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	_ "github.com/open-component-model/ocm/pkg/contexts/ocm"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	//nolint:gci // kubebuilder's scaffold imports must be appended here.
	// +kubebuilder:scaffold:imports
)

const metricCleanupTimeout = 5 * time.Minute

var (
	scheme       = machineryruntime.NewScheme() //nolint:gochecknoglobals // scheme used to add CRDs
	setupLog     = ctrl.Log.WithName("setup")   //nolint:gochecknoglobals // logger used for setup
	buildVersion = "not_provided"               //nolint:gochecknoglobals // used to embed static binary version during release builds
)

func registerSchemas() {
	machineryutilruntime.Must(k8sclientscheme.AddToScheme(scheme))
	machineryutilruntime.Must(api.AddToScheme(scheme))

	machineryutilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	machineryutilruntime.Must(certmanagerv1.AddToScheme(scheme))

	machineryutilruntime.Must(istioclientapiv1beta1.AddToScheme(scheme))

	machineryutilruntime.Must(v1beta2.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	registerSchemas()

	flagVar := flags.DefineFlagVar()
	flag.Parse()

	ctrl.SetLogger(log.ConfigLogger(int8(flagVar.LogLevel), zapcore.Lock(os.Stdout)))
	setupLog.Info("starting Lifecycle-Manager version: " + buildVersion)
	if err := flagVar.Validate(); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	if flagVar.Pprof {
		go pprofStartServer(flagVar.PprofAddr, flagVar.PprofServerTimeout)
	}

	setupManager(flagVar, internal.DefaultCacheOptions(), scheme)
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

func setupManager(flagVar *flags.FlagVar, cacheOptions cache.Options, scheme *machineryruntime.Scheme) {
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
			Cache:                  cacheOptions,
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var skrWebhookManager *watcher.SKRWebhookManifestManager
	options := controllerOptionsFromFlagVar(flagVar)
	if flagVar.EnableKcpWatcher {
		if skrWebhookManager, err = createSkrWebhookManager(mgr, flagVar); err != nil {
			setupLog.Error(err, "failed to create skr webhook manager")
			os.Exit(1)
		}
		setupKcpWatcherReconciler(mgr, options, flagVar)
	}

	remoteClientCache := remote.NewClientCache()
	sharedMetrics := metrics.NewSharedMetrics()
	descriptorProvider := provider.NewCachedDescriptorProvider(nil)
	kymaMetrics := metrics.NewKymaMetrics(sharedMetrics)
	setupKymaReconciler(mgr, remoteClientCache, descriptorProvider, flagVar, options, skrWebhookManager, kymaMetrics)
	setupManifestReconciler(mgr, flagVar, options, sharedMetrics)
	setupMandatoryModuleReconciler(mgr, descriptorProvider, flagVar, options)
	setupMandatoryModuleDeletionReconciler(mgr, descriptorProvider, flagVar, options)

	if flagVar.EnablePurgeFinalizer {
		setupPurgeReconciler(mgr, remoteClientCache, flagVar, options)
	}
	if flagVar.EnableWebhooks {
		enableWebhooks(mgr)
	}

	addHealthChecks(mgr)
	if flagVar.DropStoredVersion != "" {
		go func(version string) {
			dropStoredVersion(mgr, version)
		}(flagVar.DropStoredVersion)
	}
	go runKymaMetricsCleanup(kymaMetrics, mgr.GetClient(), flagVar.MetricsCleanupIntervalInMinutes)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func addHealthChecks(mgr manager.Manager) {
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

func runKymaMetricsCleanup(kymaMetrics *metrics.KymaMetrics, kcpClient client.Client,
	cleanupIntervalInMinutes int,
) {
	scheduler := gocron.NewScheduler(time.UTC)
	_, scheduleErr := scheduler.Every(cleanupIntervalInMinutes).Minutes().Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), metricCleanupTimeout)
		defer cancel()
		if err := kymaMetrics.CleanupNonExistingKymaCrsMetrics(ctx, kcpClient); err != nil {
			setupLog.Info(fmt.Sprintf("failed to cleanup non existing kyma crs metrics, err: %s", err))
		}
	})
	if scheduleErr != nil {
		setupLog.Info(fmt.Sprintf("failed to setup cleanup routine for non existing kyma crs metrics, err: %s",
			scheduleErr))
	}
	scheduler.StartAsync()
}

func enableWebhooks(mgr manager.Manager) {
	if err := (&v1beta2.ModuleTemplate{}).
		SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ModuleTemplate")
		os.Exit(1)
	}

	if err := (&v1beta2.Kyma{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Kyma")
		os.Exit(1)
	}
	if err := (&v1beta2.Watcher{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Watcher")
		os.Exit(1)
	}

	if err := (&v1beta2.Manifest{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Manifest")
		os.Exit(1)
	}
}

func controllerOptionsFromFlagVar(flagVar *flags.FlagVar) ctrlruntime.Options {
	return ctrlruntime.Options{
		RateLimiter: workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(flagVar.FailureBaseDelay, flagVar.FailureMaxDelay),
			&workqueue.BucketRateLimiter{
				Limiter: rate.NewLimiter(rate.Limit(flagVar.RateLimiterFrequency), flagVar.RateLimiterBurst),
			},
		),

		CacheSyncTimeout: flagVar.CacheSyncTimeout,
	}
}

func setupKymaReconciler(mgr ctrl.Manager, remoteClientCache *remote.ClientCache,
	descriptorProvider *provider.CachedDescriptorProvider,
	flagVar *flags.FlagVar, options ctrlruntime.Options, skrWebhookManager *watcher.SKRWebhookManifestManager,
	kymaMetrics *metrics.KymaMetrics,
) {
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentKymaReconciles
	kcpRestConfig := mgr.GetConfig()

	if err := (&controller.KymaReconciler{
		Client:             mgr.GetClient(),
		EventRecorder:      mgr.GetEventRecorderFor(shared.OperatorName),
		KcpRestConfig:      kcpRestConfig,
		RemoteClientCache:  remoteClientCache,
		DescriptorProvider: descriptorProvider,
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
	}).SetupWithManager(
		mgr, options, controller.SetupUpSetting{
			ListenerAddr:                 flagVar.KymaListenerAddr,
			EnableDomainNameVerification: flagVar.EnableDomainNameVerification,
			IstioNamespace:               flagVar.IstioNamespace,
		},
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kyma")
		os.Exit(1)
	}
}

func createSkrWebhookManager(mgr ctrl.Manager, flagVar *flags.FlagVar) (*watcher.SKRWebhookManifestManager, error) {
	caCertificateCache := watcher.NewCACertificateCache(flagVar.CaCertCacheTTL)
	config := watcher.SkrWebhookManagerConfig{
		SKRWatcherPath:         flagVar.WatcherResourcesPath,
		SkrWatcherImage:        getWatcherImg(flagVar),
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
	}
	gatewayConfig := watcher.GatewayConfig{
		IstioGatewayName:          flagVar.IstioGatewayName,
		IstioGatewayNamespace:     flagVar.IstioGatewayNamespace,
		LocalGatewayPortOverwrite: flagVar.ListenerPortOverwrite,
	}
	return watcher.NewSKRWebhookManifestManager(
		mgr.GetConfig(),
		mgr.GetScheme(),
		caCertificateCache,
		config,
		certConfig,
		gatewayConfig)
}

const (
	watcherRegProd = "europe-docker.pkg.dev/kyma-project/prod/runtime-watcher-skr"
	watcherRegDev  = "europe-docker.pkg.dev/kyma-project/dev/runtime-watcher"
)

func getWatcherImg(flagVar *flags.FlagVar) string {
	if flagVar.UseWatcherDevRegistry {
		return fmt.Sprintf("%s:%s", watcherRegDev, flagVar.WatcherImageTag)
	}
	return fmt.Sprintf("%s:%s", watcherRegProd, flagVar.WatcherImageTag)
}

func setupPurgeReconciler(mgr ctrl.Manager,
	remoteClientCache *remote.ClientCache,
	flagVar *flags.FlagVar,
	options ctrlruntime.Options,
) {
	resolveRemoteClientFunc := func(ctx context.Context, key client.ObjectKey) (client.Client, error) {
		kcpClient := remote.NewClientWithConfig(mgr.GetClient(), mgr.GetConfig())
		return remote.NewClientLookup(kcpClient, remoteClientCache, v1beta2.SyncStrategyLocalSecret).Lookup(ctx, key)
	}

	if err := (&controller.PurgeReconciler{
		Client:                mgr.GetClient(),
		EventRecorder:         mgr.GetEventRecorderFor(shared.OperatorName),
		ResolveRemoteClient:   resolveRemoteClientFunc,
		PurgeFinalizerTimeout: flagVar.PurgeFinalizerTimeout,
		SkipCRDs:              matcher.CreateCRDMatcherFrom(flagVar.SkipPurgingFor),
		IsManagedKyma:         flagVar.IsKymaManaged,
		Metrics:               metrics.NewPurgeMetrics(),
	}).SetupWithManager(
		mgr, options,
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PurgeReconciler")
		os.Exit(1)
	}
}

func setupManifestReconciler(mgr ctrl.Manager, flagVar *flags.FlagVar, options ctrlruntime.Options,
	sharedMetrics *metrics.SharedMetrics,
) {
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentManifestReconciles
	options.RateLimiter = internal.ManifestRateLimiter(flagVar.FailureBaseDelay,
		flagVar.FailureMaxDelay, flagVar.RateLimiterFrequency, flagVar.RateLimiterBurst)

	if err := controller.SetupWithManager(
		mgr, options, queue.RequeueIntervals{
			Success: flagVar.ManifestRequeueSuccessInterval,
			Busy:    flagVar.KymaRequeueBusyInterval,
		}, controller.SetupUpSetting{
			ListenerAddr:                 flagVar.ManifestListenerAddr,
			EnableDomainNameVerification: flagVar.EnableDomainNameVerification,
		}, metrics.NewManifestMetrics(sharedMetrics),
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Manifest")
		os.Exit(1)
	}
}

func setupKcpWatcherReconciler(mgr ctrl.Manager, options ctrlruntime.Options, flagVar *flags.FlagVar) {
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentWatcherReconciles

	if err := (&controller.WatcherReconciler{
		Client:             mgr.GetClient(),
		EventRecorder:      mgr.GetEventRecorderFor(shared.OperatorName),
		WatcherVSNamespace: flagVar.IstioGatewayNamespace,
		Scheme:             mgr.GetScheme(),
		RestConfig:         mgr.GetConfig(),
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.WatcherRequeueSuccessInterval,
			Busy:    flags.DefaultKymaRequeueBusyInterval,
			Error:   flags.DefaultKymaRequeueErrInterval,
			Warning: flags.DefaultKymaRequeueWarningInterval,
		},
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", controller.WatcherControllerName)
		os.Exit(1)
	}
}

func setupMandatoryModuleReconciler(mgr ctrl.Manager, descriptorProvider *provider.CachedDescriptorProvider,
	flagVar *flags.FlagVar, options ctrlruntime.Options,
) {
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentMandatoryModuleReconciles

	if err := (&controller.MandatoryModuleReconciler{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(shared.OperatorName),
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.MandatoryModuleRequeueSuccessInterval,
			Busy:    flagVar.KymaRequeueBusyInterval,
			Error:   flagVar.KymaRequeueErrInterval,
			Warning: flagVar.KymaRequeueWarningInterval,
		},
		RemoteSyncNamespace: flagVar.RemoteSyncNamespace,
		InKCPMode:           flagVar.InKCPMode,
		DescriptorProvider:  descriptorProvider,
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MandatoryModule")
		os.Exit(1)
	}
}

func setupMandatoryModuleDeletionReconciler(mgr ctrl.Manager, descriptorProvider *provider.CachedDescriptorProvider,
	flagVar *flags.FlagVar, options ctrlruntime.Options,
) {
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentMandatoryModuleDeletionReconciles

	if err := (&controller.MandatoryModuleDeletionReconciler{
		Client:             mgr.GetClient(),
		EventRecorder:      mgr.GetEventRecorderFor(shared.OperatorName),
		DescriptorProvider: descriptorProvider,
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.MandatoryModuleDeletionRequeueSuccessInterval,
			Busy:    flagVar.KymaRequeueBusyInterval,
			Error:   flagVar.KymaRequeueErrInterval,
			Warning: flagVar.KymaRequeueWarningInterval,
		},
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MandatoryModule")
		os.Exit(1)
	}
}

func dropStoredVersion(mgr manager.Manager, versionToBeRemoved string) {
	cfg := mgr.GetConfig()
	kcpClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		setupLog.V(log.DebugLevel).Error(err,
			fmt.Sprintf("unable to initialize client to remove %s", versionToBeRemoved))
	}
	ctx := context.TODO()
	var crdList *apiextensionsv1.CustomResourceDefinitionList
	if crdList, err = kcpClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx,
		apimetav1.ListOptions{}); err != nil {
		setupLog.V(log.InfoLevel).Error(err, "unable to list CRDs")
	}

	crdsToPatch := []string{
		string(shared.ModuleTemplateKind), string(shared.WatcherKind),
		string(shared.ManifestKind), string(shared.KymaKind),
	}

	for _, crdItem := range crdList.Items {
		if crdItem.Spec.Group != shared.OperatorGroup && !slices.Contains(crdsToPatch, crdItem.Spec.Names.Kind) {
			continue
		}
		setupLog.V(log.InfoLevel).Info(fmt.Sprintf("Checking the storedVersions for %s crd", crdItem.Spec.Names.Kind))
		oldStoredVersions := crdItem.Status.StoredVersions
		newStoredVersions := make([]string, 0, len(oldStoredVersions))
		for _, stored := range oldStoredVersions {
			if stored != versionToBeRemoved {
				newStoredVersions = append(newStoredVersions, stored)
			}
		}
		crdItem.Status.StoredVersions = newStoredVersions
		setupLog.V(log.InfoLevel).Info(fmt.Sprintf("The new storedVersions are %v", newStoredVersions))
		crd := crdItem
		if _, err := kcpClient.ApiextensionsV1().CustomResourceDefinitions().
			UpdateStatus(ctx, &crd, apimetav1.UpdateOptions{}); err != nil {
			msg := fmt.Sprintf("Failed to update CRD to remove %s from stored versions", versionToBeRemoved)
			setupLog.V(log.InfoLevel).Error(err, msg)
		}
	}
}
