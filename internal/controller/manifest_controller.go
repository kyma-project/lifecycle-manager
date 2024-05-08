package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/security"
)

func SetupWithManager(mgr manager.Manager,
	options ctrlruntime.Options,
	requeueIntervals queue.RequeueIntervals,
	settings SetupUpSetting,
	manifestMetrics *metrics.ManifestMetrics, mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
) error {
	var verifyFunc watcherevent.Verify
	if settings.EnableDomainNameVerification {
		// Verifier used to verify incoming listener requests
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}

	runnableListener := watcherevent.NewSKREventListener(
		settings.ListenerAddr, strings.ToLower(declarativev2.OperatorName),
		verifyFunc,
	)

	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
		return fmt.Errorf("failed to add to listener to manager: %w", err)
	}

	addSkrEventToQueueFunc := &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
			queue workqueue.RateLimitingInterface,
		) {
			ctrl.Log.WithName("listener").Info(
				fmt.Sprintf(
					"event coming from SKR, adding %s to queue",
					client.ObjectKeyFromObject(evnt.Object).String(),
				),
			)
			queue.Add(ctrl.Request{NamespacedName: client.ObjectKeyFromObject(evnt.Object)})
		},
	}

	skrEventChannel := source.Channel(runnableListener.ReceivedEvents, addSkrEventToQueueFunc)

	controllerManagedByManager := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Named(ManifestControllerName).
		Watches(&apicorev1.Secret{}, handler.Funcs{}).
		WatchesRawSource(skrEventChannel).
		WithOptions(options)

	if err := controllerManagedByManager.Complete(ManifestReconciler(mgr, requeueIntervals,
		manifestMetrics, mandatoryModulesMetrics)); err != nil {
		return fmt.Errorf("failed to initialize manifest controller by manager: %w", err)
	}
	return nil
}

func ManifestReconciler(mgr manager.Manager, requeueIntervals queue.RequeueIntervals,
	manifestMetrics *metrics.ManifestMetrics, mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
) *declarativev2.Reconciler {
	kcp := &declarativev2.ClusterInfo{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}
	extractor := manifest.NewPathExtractor(nil)
	lookup := &manifest.RemoteClusterLookup{KCP: kcp}
	return declarativev2.NewFromManager(
		mgr, &v1beta2.Manifest{}, requeueIntervals, manifestMetrics, mandatoryModulesMetrics,
		declarativev2.WithSpecResolver(
			manifest.NewSpecResolver(kcp, extractor),
		),
		declarativev2.WithCustomReadyCheck(manifest.NewCustomResourceReadyCheck()),
		declarativev2.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
		declarativev2.WithPostRun{manifest.PostRunCreateCR},
		declarativev2.WithPreDelete{manifest.PreDeleteDeleteCR},
		declarativev2.WithModuleCRDeletionCheck(manifest.NewModuleCRDeletionCheck()),
	)
}
