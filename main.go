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
	"time"

	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"

	certManagerV1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/open-component-model/ocm/pkg/contexts/oci"
	"github.com/open-component-model/ocm/pkg/contexts/oci/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/strings/slices"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	operatorv1beta2 "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	//+kubebuilder:scaffold:imports
	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
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
	ocm.DefaultContext().RepositoryTypes().Register(
		genericocireg.Type, genericocireg.NewRepositoryType(oci.DefaultContext()),
	)
	ocm.DefaultContext().RepositoryTypes().Register(
		genericocireg.TypeV1, genericocireg.NewRepositoryType(oci.DefaultContext()),
	)
	cpi.DefaultContext().RepositoryTypes().Register(
		ocireg.LegacyType, genericocireg.NewRepositoryType(oci.DefaultContext()),
	)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(api.AddToScheme(scheme))

	utilruntime.Must(v1extensions.AddToScheme(scheme))
	utilruntime.Must(certManagerV1.AddToScheme(scheme))

	utilruntime.Must(istiov1beta1.AddToScheme(scheme))

	utilruntime.Must(operatorv1beta2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	flagVar := defineFlagVar()
	flag.Parse()
	ctrl.SetLogger(log.ConfigLogger(int8(flagVar.logLevel), zapcore.Lock(os.Stdout)))
	if flagVar.pprof {
		go pprofStartServer(flagVar.pprofAddr, flagVar.pprofServerTimeout)
	}

	setupManager(flagVar, controllers.NewCacheOptions(), scheme)
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

func setupManager(flagVar *FlagVar, newCacheOptions cache.Options, scheme *runtime.Scheme) {
	config := ctrl.GetConfigOrDie()
	config.QPS = float32(flagVar.clientQPS)
	config.Burst = flagVar.clientBurst

	mgr, err := ctrl.NewManager(
		config, ctrl.Options{
			Scheme:                 scheme,
			MetricsBindAddress:     flagVar.metricsAddr,
			Port:                   port,
			HealthProbeBindAddress: flagVar.probeAddr,
			LeaderElection:         flagVar.enableLeaderElection,
			LeaderElectionID:       "893110f7.kyma-project.io",
			Cache:                  newCacheOptions,
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	options := controllerOptionsFromFlagVar(flagVar)

	remoteClientCache := remote.NewClientCache()

	setupKymaReconciler(mgr, remoteClientCache, flagVar, options)
	setupManifestReconciler(mgr, flagVar, options)

	if flagVar.enableKcpWatcher {
		setupKcpWatcherReconciler(mgr, options, flagVar)
	}
	if flagVar.enableWebhooks {
		enableWebhooks(mgr)
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

	go func() {
		dropVersionFromStoredVersions(mgr, "v1alpha1")
	}()

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func enableWebhooks(mgr manager.Manager) {
	if err := (&operatorv1beta2.ModuleTemplate{}).
		SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ModuleTemplate")
		os.Exit(1)
	}

	if err := (&operatorv1beta2.Kyma{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Kyma")
		os.Exit(1)
	}
	if err := (&operatorv1beta2.Watcher{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Watcher")
		os.Exit(1)
	}

	if err := (&operatorv1beta2.Manifest{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Manifest")
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

		CacheSyncTimeout: flagVar.cacheSyncTimeout,
	}
}

func setupKymaReconciler(mgr ctrl.Manager,
	remoteClientCache *remote.ClientCache,
	flagVar *FlagVar, options controller.Options,
) {
	options.MaxConcurrentReconciles = flagVar.maxConcurrentKymaReconciles
	kcpRestConfig := mgr.GetConfig()
	var skrWebhookManager watcher.SKRWebhookManager
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
	}

	if err := (&controllers.KymaReconciler{
		Client:            mgr.GetClient(),
		EventRecorder:     mgr.GetEventRecorderFor(operatorv1beta2.OperatorName),
		KcpRestConfig:     kcpRestConfig,
		RemoteClientCache: remoteClientCache,
		SKRWebhookManager: skrWebhookManager,
		RequeueIntervals: controllers.RequeueIntervals{
			Success: flagVar.kymaRequeueSuccessInterval,
			Busy:    flagVar.kymaRequeueBusyInterval,
			Error:   flagVar.kymaRequeueErrInterval,
		},
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: flagVar.enableVerification,
			PublicKeyFilePath:  flagVar.moduleVerificationKeyFilePath,
		},
		InKCPMode:           flagVar.inKCPMode,
		RemoteSyncNamespace: flagVar.remoteSyncNamespace,
		IsManagedKyma:       flagVar.isKymaManaged,
	}).SetupWithManager(
		mgr, options, controllers.SetupUpSetting{
			ListenerAddr:                 flagVar.kymaListenerAddr,
			EnableDomainNameVerification: flagVar.enableDomainNameVerification,
			IstioNamespace:               flagVar.istioNamespace,
		},
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kyma")
		os.Exit(1)
	}
	if flagVar.enablePurgeFinalizer {
		setupPurgeReconciler(mgr, remoteClientCache, flagVar, options, kcpRestConfig)
	}
	metrics.Initialize()
}

func createSkrWebhookManager(mgr ctrl.Manager, flagVar *FlagVar) (watcher.SKRWebhookManager, error) {
	return watcher.NewSKRWebhookManifestManager(mgr.GetClient(), &watcher.SkrWebhookManagerConfig{
		SKRWatcherPath:              flagVar.skrWatcherPath,
		SkrWatcherImage:             flagVar.skrWatcherImage,
		SkrWebhookCPULimits:         flagVar.skrWebhookCPULimits,
		SkrWebhookMemoryLimits:      flagVar.skrWebhookMemoryLimits,
		WatcherLocalTestingEnabled:  flagVar.enableWatcherLocalTesting,
		LocalGatewayHTTPPortMapping: flagVar.listenerHTTPSPortLocalMapping,
		IstioNamespace:              flagVar.istioNamespace,
		IstioGatewayName:            flagVar.istioGatewayName,
		IstioGatewayNamespace:       flagVar.istioGatewayNamespace,
		RemoteSyncNamespace:         flagVar.remoteSyncNamespace,
	})
}

func setupPurgeReconciler(
	mgr ctrl.Manager,
	remoteClientCache *remote.ClientCache,
	flagVar *FlagVar,
	options controller.Options,
	restConfig *rest.Config,
) {
	resolveRemoteClientFunc := func(ctx context.Context, key client.ObjectKey) (client.Client, error) {
		kcpClient := remote.NewClientWithConfig(mgr.GetClient(), restConfig)
		return remote.NewClientLookup(kcpClient, remoteClientCache, operatorv1beta2.SyncStrategyLocalSecret).Lookup(ctx, key)
	}

	if err := (&controllers.PurgeReconciler{
		Client:                mgr.GetClient(),
		EventRecorder:         mgr.GetEventRecorderFor(operatorv1beta2.OperatorName),
		ResolveRemoteClient:   resolveRemoteClientFunc,
		PurgeFinalizerTimeout: flagVar.purgeFinalizerTimeout,
		SkipCRDs:              controllers.CRDMatcherFor(flagVar.skipPurgingFor),
		IsManagedKyma:         flagVar.isKymaManaged,
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
	options controller.Options,
) {
	options.MaxConcurrentReconciles = flagVar.maxConcurrentManifestReconciles
	options.RateLimiter = internal.ManifestRateLimiter(flagVar.failureBaseDelay,
		flagVar.failureMaxDelay, flagVar.rateLimiterFrequency, flagVar.rateLimiterBurst)

	if err := controllers.SetupWithManager(
		mgr, options, flagVar.manifestRequeueSuccessInterval, controllers.SetupUpSetting{
			ListenerAddr:                 flagVar.manifestListenerAddr,
			EnableDomainNameVerification: flagVar.enableDomainNameVerification,
		},
	); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Manifest")
		os.Exit(1)
	}
}

func setupKcpWatcherReconciler(mgr ctrl.Manager, options controller.Options, flagVar *FlagVar) {
	options.MaxConcurrentReconciles = flagVar.maxConcurrentWatcherReconciles

	istioConfig := istio.NewConfig(flagVar.enableWatcherLocalTesting)

	if err := (&controllers.WatcherReconciler{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(controllers.WatcherControllerName),
		Scheme:        mgr.GetScheme(),
		RestConfig:    mgr.GetConfig(),
		RequeueIntervals: controllers.RequeueIntervals{
			Success: flagVar.watcherRequeueSuccessInterval,
		},
	}).SetupWithManager(mgr, options, istioConfig); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", controllers.WatcherControllerName)
		os.Exit(1)
	}
}

func dropVersionFromStoredVersions(mgr manager.Manager, versionToBeRemoved string) {
	cfg := mgr.GetConfig()
	kcpClient, err := apiextension.NewForConfig(cfg)
	if err != nil {
		setupLog.V(log.DebugLevel).Error(err, fmt.Sprintf("unable to initialize client to remove %s", versionToBeRemoved))
	}
	ctx := context.TODO()
	var crdList *v1extensions.CustomResourceDefinitionList
	if crdList, err = kcpClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, v1.ListOptions{}); err != nil {
		setupLog.V(log.InfoLevel).Error(err, "unable to list CRDs")
	}

	crdsToPatch := []string{
		string(operatorv1beta2.ModuleTemplateKind), string(operatorv1beta2.WatcherKind),
		operatorv1beta2.ManifestKind, string(operatorv1beta2.KymaKind),
	}

	for _, crdItem := range crdList.Items {
		if crdItem.Spec.Group != "operator.kyma-project.io" && !slices.Contains(crdsToPatch, crdItem.Spec.Names.Kind) {
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
			UpdateStatus(ctx, &crd, v1.UpdateOptions{}); err != nil {
			msg := fmt.Sprintf("Failed to update CRD to remove %s from stored versions", versionToBeRemoved)
			setupLog.V(log.InfoLevel).Error(err, msg)
		}
	}
}
