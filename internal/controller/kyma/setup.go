package kyma

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/v2/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
	"github.com/kyma-project/lifecycle-manager/pkg/security"
)

type SetupOptions struct {
	ListenerAddr                 string
	EnableDomainNameVerification bool
	IstioNamespace               string
}

const controllerName = "kyma"

var (
	errConvertingWatcherEvent = errors.New("error converting watched object to unstructured event")
	errParsingRuntimeID       = errors.New("error getting runtime id from unstructured event")
)

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options, settings SetupOptions) error {
	var verifyFunc watcherevent.Verify
	if settings.EnableDomainNameVerification {
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}
	runnableListener := watcherevent.NewSKREventListener(
		settings.ListenerAddr,
		shared.OperatorName,
		verifyFunc,
	)

	if err := mgr.Add(runnableListener); err != nil {
		return fmt.Errorf("KymaReconciler %w", err)
	}

	if err := ctrl.NewControllerManagedBy(mgr).For(&v1beta2.Kyma{}).
		Named(controllerName).
		WithOptions(opts).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})).
		Watches(&v1beta2.ModuleTemplate{},
			handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch())).
		Watches(&v1beta2.ModuleReleaseMeta{}, watch.NewModuleReleaseMetaEventHandler(r)).
		Watches(&apicorev1.Secret{}, handler.Funcs{}).
		Watches(&v1beta2.Manifest{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &v1beta2.Kyma{},
				handler.OnlyControllerOwner()), builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		WatchesRawSource(source.Channel(controller.AdaptEvents(runnableListener.ReceivedEvents), r.skrEventHandler())).
		Complete(r); err != nil {
		return fmt.Errorf("failed to setup manager for kyma controller: %w", err)
	}

	return nil
}

func (r *Reconciler) skrEventHandler() *handler.Funcs {
	return &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
			queue workqueue.TypedRateLimitingInterface[ctrl.Request],
		) {
			logger := ctrl.Log.WithName("listener")
			unstructWatcherEvt, conversionOk := evnt.Object.(*unstructured.Unstructured)
			if !conversionOk {
				logger.Error(errConvertingWatcherEvent, fmt.Sprintf("event: %v", evnt.Object))
				return
			}

			runtimeID, ok := unstructWatcherEvt.Object["runtime-id"].(string)
			if !ok {
				logger.Error(errParsingRuntimeID, fmt.Sprintf("unstructured event: %v", unstructWatcherEvt))
				return
			}

			ownerObjectKey := client.ObjectKey{
				Name:      runtimeID,
				Namespace: shared.DefaultControlPlaneNamespace,
			}

			logger.Info(
				fmt.Sprintf("event received from SKR, adding %s to queue",
					ownerObjectKey),
			)

			queue.Add(ctrl.Request{
				NamespacedName: ownerObjectKey,
			})
		},
	}
}
