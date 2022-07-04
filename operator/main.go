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
	"os"
	"time"

	"go.uber.org/zap/zapcore" //nolint:gci
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/controllers"
	operatorLabels "github.com/kyma-project/kyma-operator/operator/pkg/labels"
	//+kubebuilder:scaffold:imports
)

const (
	name                          = "kyma-operator"
	baseDelay                     = 100 * time.Millisecond
	maxDelay                      = 1000 * time.Second
	limit                         = rate.Limit(30)
	burst                         = 200
	port                          = 9443
	defaultRequeueSuccessInterval = 20 * time.Second
	defaultRequeueFailureInterval = 10 * time.Second
	defaultRequeueWaitingInterval = 3 * time.Second
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

//nolint:wsl
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(v1extensions.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string

	var enableLeaderElection bool

	var probeAddr string

	var maxConcurrentReconciles int

	var requeueSuccessInterval, requeueFailureInterval, requeueWaitingInterval time.Duration

	defineFlag(metricsAddr, probeAddr, maxConcurrentReconciles,
		enableLeaderElection, requeueSuccessInterval, requeueFailureInterval, requeueWaitingInterval)

	opts := zap.Options{
		Development: true,
		Level:       zapcore.DebugLevel,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	cacheLabelSelector := labels.SelectorFromSet(
		labels.Set{operatorLabels.ManagedBy: name},
	)

	mgr := setupManager(metricsAddr, probeAddr, enableLeaderElection,
		cacheLabelSelector, requeueSuccessInterval, requeueFailureInterval,
		requeueWaitingInterval, maxConcurrentReconciles)
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

func setupManager(metricsAddr string, probeAddr string, enableLeaderElection bool,
	cacheLabelSelector labels.Selector, requeueSuccessInterval time.Duration,
	requeueFailureInterval time.Duration, requeueWaitingInterval time.Duration, maxConcurrentReconciles int,
) manager.Manager {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   port,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "893110f7.kyma-project.io",
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				&operatorv1alpha1.ModuleTemplate{}: {Label: cacheLabelSelector},
				&corev1.Secret{}:                   {Label: cacheLabelSelector},
			},
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.KymaReconciler{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetEventRecorderFor(name),
		RequeueIntervals: controllers.RequeueIntervals{
			Success: requeueSuccessInterval,
			Failure: requeueFailureInterval,
			Waiting: requeueWaitingInterval,
		},
	}).SetupWithManager(mgr, controller.Options{
		RateLimiter: workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(baseDelay, maxDelay),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(limit, burst)}),
		MaxConcurrentReconciles: maxConcurrentReconciles,
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kyma")
		os.Exit(1)
	}

	return mgr
}

func defineFlag(metricsAddr string, probeAddr string, maxConcurrentReconciles int,
	enableLeaderElection bool, requeueSuccessInterval time.Duration,
	requeueFailureInterval time.Duration, requeueWaitingInterval time.Duration,
) {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1, "The maximum number of concurrent Reconciles which can be run.") //nolint:lll
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&requeueSuccessInterval, "requeue-success-interval", defaultRequeueSuccessInterval,
		"determines the duration after which an already successfully reconciled Kyma is enqueued for checking "+
			"if it's still in a consistent state.")
	flag.DurationVar(&requeueFailureInterval, "requeue-failure-interval", defaultRequeueFailureInterval,
		"determines the duration after which a failing reconciliation is retried and "+
			"enqueued for a next try at recovering (e.g. because an Remote Synchronization Interaction failed)")
	flag.DurationVar(&requeueWaitingInterval, "requeue-waiting-interval", defaultRequeueWaitingInterval,
		"etermines the duration after which a pending reconciliation is requeued "+
			"if the operator decides that it needs to wait for a certain state to update before it can proceed "+
			"(e.g. because of pending finalizers in the deletion process)")
}
