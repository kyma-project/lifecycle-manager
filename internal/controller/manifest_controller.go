package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
	apicore "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/security"
)

func SetupWithManager(
	mgr manager.Manager,
	options ctrlruntime.Options,
	checkInterval time.Duration,
	settings SetupUpSetting,
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

	runnableListener, eventChannel := watcherevent.RegisterListenerComponent(
		settings.ListenerAddr, strings.ToLower(declarative.OperatorName), verifyFunc,
	)

	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
		return fmt.Errorf("failed to add to listener to manager: %w", err)
	}

	controllerManagedByManager := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Watches(&apicore.Secret{}, handler.Funcs{}).
		WatchesRawSource(
			eventChannel, &handler.Funcs{
				GenericFunc: func(ctx context.Context, event event.GenericEvent, queue workqueue.RateLimitingInterface) {
					ctrl.Log.WithName("listener").Info(
						fmt.Sprintf(
							"event coming from SKR, adding %s to queue",
							client.ObjectKeyFromObject(event.Object).String(),
						),
					)
					queue.Add(ctrl.Request{NamespacedName: client.ObjectKeyFromObject(event.Object)})
				},
			},
		).WithOptions(options)

	if err := controllerManagedByManager.Complete(ManifestReconciler(mgr, checkInterval)); err != nil {
		return fmt.Errorf("failed to initialize manifest controller by manager: %w", err)
	}
	return nil
}

func ManifestReconciler(
	mgr manager.Manager,
	checkInterval time.Duration,
) *declarative.Reconciler {
	kcp := &declarative.ClusterInfo{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}
	lookup := &manifest.RemoteClusterLookup{KCP: kcp}
	return declarative.NewFromManager(
		mgr, &v1beta2.Manifest{},
		declarative.WithSpecResolver(
			manifest.NewSpecResolver(kcp),
		),
		declarative.WithCustomReadyCheck(manifest.NewCustomResourceReadyCheck()),
		declarative.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
		declarative.WithPostRun{manifest.PostRunCreateCR},
		declarative.WithPreDelete{manifest.PreDeleteDeleteCR},
		declarative.WithPeriodicConsistencyCheck(checkInterval),
		declarative.WithModuleCRDeletionCheck(manifest.NewModuleCRDeletionCheck()),
	)
}
