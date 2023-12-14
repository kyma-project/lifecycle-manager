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
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/matcher"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	_ "github.com/open-component-model/ocm/pkg/contexts/ocm"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	//nolint:gci // kubebuilder's scaffold imports must be appended here.
	// +kubebuilder:scaffold:imports
)

var (
	scheme                 = machineryruntime.NewScheme() //nolint:gochecknoglobals // scheme used to add CRDs
	setupLog               = ctrl.Log.WithName("setup")   //nolint:gochecknoglobals // logger used for setup
	errMissingWatcherImage = errors.New("runtime watcher image is not provided")
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

	flagVar := DefineFlagVar()
	flag.Parse()
	ctrl.SetLogger(log.ConfigLogger(int8(flagVar.logLevel), zapcore.Lock(os.Stdout)))
	if flagVar.pprof {
		go pprofStartServer(flagVar.pprofAddr, flagVar.pprofServerTimeout)
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

func setupManager(flagVar *FlagVar, cacheOptions cache.Options, scheme *machineryruntime.Scheme) {
	config := ctrl.GetConfigOrDie()
	config.QPS = float32(flagVar.clientQPS)
	config.Burst = flagVar.clientBurst

	mgr, err := ctrl.NewManager(
		config, ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: flagVar.metricsAddr,
			},
			HealthProbeBindAddress: flagVar.probeAddr,
			LeaderElection:         flagVar.enableLeaderElection,
			LeaderElectionID:       "893110f7.kyma-project.io",
			Cache:                  cacheOptions,
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	options := controllerOptionsFromFlagVar(flagVar)
	if flagVar.enableKcpWatcher && flagVar.skrWatcherImage == "" {
		setupLog.Error(errMissingWatcherImage, "unable to start manager")
		os.Exit(1)
	}
	remoteClientCache := remote.NewClientCache()
	var skrWebhookManager *watcher.SKRWebhookManifestManager
	if flagVar.enableKcpWatcher {
		watcherChartDirInfo, err := os.Stat(flagVar.skrWatcherPath)
		if err != nil || !watcherChartDirInfo.IsDir() {
			setupLog.Error(err, "failed to read local skr chart")
			os.Exit(1)
		}

		if skrWebhookManager, err = createSkrWebhookManager(mgr, flagVar); err != nil {
			setupLog.Error(err, "failed to create webhook chart manager")
			os.Exit(1)
		}
		setupKcpWatcherReconciler(mgr, options, flagVar)
	}

	setupKymaReconciler(mgr, remoteClientCache, flagVar, options, skrWebhookManager)
	setupManifestReconciler(mgr, flagVar, options)

	if flagVar.enablePurgeFinalizer {
		setupPurgeReconciler(mgr, remoteClientCache, flagVar, options)
	}

	if flagVar.enableWebhooks {
		enableWebhooks(mgr)
	}

	// +kubebuilder:scaffold:builder
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if flagVar.dropStoredVersion != "" {
		go func(version string) {
			dropStoredVersion(mgr, version)
		}(flagVar.dropStoredVersion)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
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

func controllerOptionsFromFlagVar(flagVar *FlagVar) ctrlruntime.Options {
	return ctrlruntime.Options{
		RateLimiter: workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(flagVar.failureBaseDelay, flagVar.failureMaxDelay),
			&workqueue.BucketRateLimiter{
				Limiter: rate.NewLimiter(rate.Limit(flagVar.rateLimiterFrequency), flagVar.rateLimiterBurst),
			},
		),

		CacheSyncTimeout: flagVar.cacheSyncTimeout,
	}
}

func setupKymaReconciler(mgr ctrl.Manager, remoteClientCache *remote.ClientCache, flagVar *FlagVar,
	options ctrlruntime.Options, skrWebhookManager *watcher.SKRWebhookManifestManager,
) {
	options.MaxConcurrentReconciles = flagVar.maxConcurrentKymaReconciles
	kcpRestConfig := mgr.GetConfig()

	if err := (&controller.KymaReconciler{
		Client:            mgr.GetClient(),
		EventRecorder:     mgr.GetEventRecorderFor(shared.OperatorName),
		KcpRestConfig:     kcpRestConfig,
		RemoteClientCache: remoteClientCache,
		SKRWebhookManager: skrWebhookManager,
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.kymaRequeueSuccessInterval,
			Busy:    flagVar.kymaRequeueBusyInterval,
			Error:   flagVar.kymaRequeueErrInterval,
			Warning: flagVar.kymaRequeueWarningInterval,
		},
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: flagVar.enableVerification,
			PublicKeyFilePath:  flagVar.moduleVerificationKeyFilePath,
		},
		InKCPMode:           flagVar.inKCPMode,
		RemoteSyncNamespace: flagVar.remoteSyncNamespace,
		IsManagedKyma:       flagVar.IsKymaManaged,
		Metrics:             metrics.NewKymaMetrics(),
	}).SetupWithManager(
		mgr, options, controller.SetupUpSetting{
			ListenerAddr:                 flagVar.kymaListenerAddr,
			EnableDomainNameVerification: flagVar.enableDomainNameVerification,
			IstioNamespace:               flagVar.istioNamespace,
		},
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kyma")
		os.Exit(1)
	}
}

func createSkrWebhookManager(mgr ctrl.Manager, flagVar *FlagVar) (*watcher.SKRWebhookManifestManager, error) {
	caCertificateCache := watcher.NewCACertificateCache(flagVar.caCertCacheTTL)
	return watcher.NewSKRWebhookManifestManager(mgr.GetConfig(), mgr.GetScheme(), caCertificateCache,
		watcher.SkrWebhookManagerConfig{
			SKRWatcherPath:         flagVar.skrWatcherPath,
			SkrWatcherImage:        flagVar.skrWatcherImage,
			SkrWebhookCPULimits:    flagVar.skrWebhookCPULimits,
			SkrWebhookMemoryLimits: flagVar.skrWebhookMemoryLimits,
			RemoteSyncNamespace:    flagVar.remoteSyncNamespace,
		}, watcher.CertificateConfig{
			IstioNamespace:      flagVar.istioNamespace,
			RemoteSyncNamespace: flagVar.remoteSyncNamespace,
			CACertificateName:   flagVar.caCertName,
			AdditionalDNSNames:  strings.Split(flagVar.additionalDNSNames, ","),
			Duration:            flagVar.SelfSignedCertDuration,
			RenewBefore:         flagVar.SelfSignedCertRenewBefore,
		}, watcher.GatewayConfig{
			IstioGatewayName:          flagVar.istioGatewayName,
			IstioGatewayNamespace:     flagVar.istioGatewayNamespace,
			LocalGatewayPortOverwrite: flagVar.listenerPortOverwrite,
		})
}

func setupPurgeReconciler(mgr ctrl.Manager,
	remoteClientCache *remote.ClientCache,
	flagVar *FlagVar,
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
		PurgeFinalizerTimeout: flagVar.purgeFinalizerTimeout,
		SkipCRDs:              matcher.CreateCRDMatcherFrom(flagVar.skipPurgingFor),
		IsManagedKyma:         flagVar.IsKymaManaged,
		Metrics:               metrics.NewPurgeMetrics(),
	}).SetupWithManager(
		mgr, options,
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PurgeReconciler")
		os.Exit(1)
	}
}

func setupManifestReconciler(
	mgr ctrl.Manager,
	flagVar *FlagVar,
	options ctrlruntime.Options,
) {
	options.MaxConcurrentReconciles = flagVar.maxConcurrentManifestReconciles
	options.RateLimiter = internal.ManifestRateLimiter(flagVar.failureBaseDelay,
		flagVar.failureMaxDelay, flagVar.rateLimiterFrequency, flagVar.rateLimiterBurst)

	if err := controller.SetupWithManager(
		mgr, options, flagVar.manifestRequeueSuccessInterval, controller.SetupUpSetting{
			ListenerAddr:                 flagVar.manifestListenerAddr,
			EnableDomainNameVerification: flagVar.enableDomainNameVerification,
		},
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Manifest")
		os.Exit(1)
	}
}

func setupKcpWatcherReconciler(mgr ctrl.Manager, options ctrlruntime.Options, flagVar *FlagVar) {
	options.MaxConcurrentReconciles = flagVar.maxConcurrentWatcherReconciles

	if err := (&controller.WatcherReconciler{
		Client:             mgr.GetClient(),
		EventRecorder:      mgr.GetEventRecorderFor(controller.WatcherControllerName),
		WatcherVSNamespace: flagVar.istioGatewayNamespace,
		Scheme:             mgr.GetScheme(),
		RestConfig:         mgr.GetConfig(),
		RequeueIntervals: queue.RequeueIntervals{
			Success: flagVar.watcherRequeueSuccessInterval,
			Busy:    defaultKymaRequeueBusyInterval,
			Error:   defaultKymaRequeueErrInterval,
			Warning: defaultKymaRequeueWarningInterval,
		},
	}).SetupWithManager(mgr, options); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", controller.WatcherControllerName)
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
