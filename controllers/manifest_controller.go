package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"

	listener "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/security"
)

func SetupWithManager(
	mgr manager.Manager,
	options controller.Options,
	checkInterval time.Duration,
	settings SetupUpSetting,
) error {
	var verifyFunc listener.Verify
	if settings.EnableDomainNameVerification {
		// Verifier used to verify incoming listener requests
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}

	runnableListener, eventChannel := listener.RegisterListenerComponent(
		settings.ListenerAddr, strings.ToLower(declarative.OperatorName), verifyFunc,
	)

	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
		return fmt.Errorf("failed to add to listener to manager: %w", err)
	}

	codec, err := v1beta2.NewCodec()
	if err != nil {
		return fmt.Errorf("unable to initialize codec: %w", err)
	}

	controllerManagedByManager := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Watches(&v1.Secret{}, handler.Funcs{}).
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

	if controllerManagedByManager.Complete(ManifestReconciler(mgr, codec, checkInterval)) != nil {
		return fmt.Errorf("failed to initialize manifest controller by manager: %w", err)
	}
	return nil
}

func ManifestReconciler(
	mgr manager.Manager, codec *v1beta2.Codec,
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
			manifest.NewSpecResolver(kcp, codec),
		),
		declarative.WithCustomReadyCheck(manifest.NewCustomResourceReadyCheck()),
		declarative.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
		declarative.WithPostRun{manifest.PostRunCreateCR},
		declarative.WithPreDelete{manifest.PreDeleteDeleteCR},
		declarative.WithPeriodicConsistencyCheck(checkInterval),
		declarative.WithModuleCRDName(manifest.GetModuleCRDName),
	)
}
