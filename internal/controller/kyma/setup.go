package kyma

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
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
	"github.com/kyma-project/lifecycle-manager/pkg/security"
	"github.com/kyma-project/lifecycle-manager/pkg/watch"
)

type ReconcilerSetupSettings struct {
	ListenerAddr                 string
	EnableDomainNameVerification bool
	IstioNamespace               string
}

const controllerName = "kyma"

var (
	errConvertingWatched      = errors.New("error converting watched to object key")
	errParsingWatched         = errors.New("error getting watched object from unstructured event")
	errConvertingWatcherEvent = errors.New("error converting watched object to unstructured event")
)

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, options ctrlruntime.Options, settings ReconcilerSetupSettings) error {
	predicates := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&v1beta2.Kyma{}).
		Named(controllerName).
		WithOptions(options).
		WithEventFilter(predicates).
		Watches(
			&v1beta2.ModuleTemplate{},
			handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch()),
			builder.WithPredicates(predicates),
		).
		// Watch secrets in lifecycle-manager so that cache notices changes
		Watches(&apicorev1.Secret{}, handler.Funcs{})

	controllerBuilder = controllerBuilder.Watches(&v1beta2.Manifest{},
		&watch.RestrictedEnqueueRequestForOwner{Log: ctrl.Log, OwnerType: &v1beta2.Kyma{}, IsController: true})

	var runnableListener *watcherevent.SKREventListener
	var eventChannel *source.Channel
	var verifyFunc watcherevent.Verify

	if settings.EnableDomainNameVerification {
		// Verifier used to verify incoming listener requests
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}
	// register listener component incl. domain name verification
	runnableListener, eventChannel = watcherevent.RegisterListenerComponent(
		settings.ListenerAddr,
		shared.OperatorName,
		verifyFunc,
	)

	r.watchEventChannel(controllerBuilder, eventChannel)
	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
		return fmt.Errorf("KymaReconciler %w", err)
	}

	if err := controllerBuilder.Complete(r); err != nil {
		return fmt.Errorf("error occurred while building controller: %w", err)
	}

	return nil
}

func (r *Reconciler) watchEventChannel(controllerBuilder *builder.Builder, eventChannel *source.Channel) {
	controllerBuilder.WatchesRawSource(eventChannel, &handler.Funcs{
		GenericFunc: func(ctx context.Context, event event.GenericEvent, queue workqueue.RateLimitingInterface) {
			logger := ctrl.Log.WithName("listener")
			unstructWatcherEvt, conversionOk := event.Object.(*unstructured.Unstructured)
			if !conversionOk {
				logger.Error(errConvertingWatcherEvent, fmt.Sprintf("event: %v", event.Object))
				return
			}

			// get owner object from unstructured event, owner = KymaCR object reference in KCP
			unstructuredOwner, ok := unstructWatcherEvt.Object["owner"]
			if !ok {
				logger.Error(errParsingWatched, fmt.Sprintf("unstructured event: %v", unstructWatcherEvt))
				return
			}

			ownerObjectKey, conversionOk := unstructuredOwner.(client.ObjectKey)
			if !conversionOk {
				logger.Error(errConvertingWatched, fmt.Sprintf("unstructured Owner object: %v", unstructuredOwner))
				return
			}

			logger.Info(
				fmt.Sprintf("event received from SKR, adding %s to queue",
					ownerObjectKey),
			)

			queue.Add(ctrl.Request{
				NamespacedName: ownerObjectKey,
			})
		},
	})
}
