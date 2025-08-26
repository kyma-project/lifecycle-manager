package kyma

import (
	"context"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrevent"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
)

// EventService defines the interface for event services consumed by this controller.
// Points to the shared common interface to avoid circular dependencies.
type EventService = common.EventService

type SetupOptions struct {
	ListenerAddr                 string
	EnableDomainNameVerification bool
	IstioNamespace               string
}

const controllerName = "kyma"

var errConvertingWatcherEvent = errors.New("error converting watched object to unstructured event")

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options, settings SetupOptions) error {
	runtimeEventService, err := skrevent.NewSKREventService(
		mgr,
		settings.ListenerAddr,
		"kyma", // Component name for Kyma controller events
		settings.EnableDomainNameVerification,
	)
	if err != nil {
		return fmt.Errorf("failed to create runtime event service: %w", err)
	}

	// Note: Service is automatically started by the manager (implements manager.Runnable)

	// Create event source for this controller using the service's channel
	runtimeEventSource := runtimeEventService.CreateEventSource(r.runtimeEventHandler())

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
		WatchesRawSource(runtimeEventSource).
		Complete(r); err != nil {
		return fmt.Errorf("failed to setup manager for kyma controller: %w", err)
	}

	return nil
}

func (r *Reconciler) runtimeEventHandler() *handler.Funcs {
	return &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
			queue workqueue.TypedRateLimitingInterface[ctrl.Request],
		) {
			logger := ctrl.Log.WithName("kyma-listener")
			unstructWatcherEvt, conversionOk := evnt.Object.(*unstructured.Unstructured)
			if !conversionOk {
				logger.Error(errConvertingWatcherEvent, fmt.Sprintf("event: %v", evnt.Object))
				return
			}

			// This is where ExtractOwnerKey is essential - it tells us WHICH Kyma to reconcile
			ownerObjectKey, err := skrevent.ExtractOwnerKey(unstructWatcherEvt)
			if err != nil {
				logger.Error(err, "failed to extract owner key from runtime watcher event")
				return
			}

			logger.Info(
				fmt.Sprintf("kyma event received from runtime-watcher, adding %s to queue",
					ownerObjectKey),
			)

			// The extracted owner key becomes the reconciliation target
			queue.Add(ctrl.Request{
				NamespacedName: ownerObjectKey,
			})
		},
	}
}
