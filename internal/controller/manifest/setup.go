package manifest

import (
	"context"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/orphan"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrevent"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

// EventService defines the interface for event services consumed by this controller.
// Points to the shared common interface to avoid circular dependencies.
type EventService = common.EventService

const controllerName = "manifest"

type SetupOptions struct {
	ListenerAddr                 string
	EnableDomainNameVerification bool
}

func SetupWithManager(mgr manager.Manager, opts ctrlruntime.Options, requeueIntervals queue.RequeueIntervals,
	settings SetupOptions, manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics, manifestClient declarativev2.ManifestAPIClient,
	orphanDetectionClient orphan.DetectionRepository, specResolver *spec.Resolver,
) error {
	// Create runtime event service directly - simple and clean
	runtimeEventService, err := skrevent.NewSKREventService(
		mgr,
		settings.ListenerAddr,
		"manifest", // Component name for Manifest controller events
		settings.EnableDomainNameVerification,
	)
	if err != nil {
		return fmt.Errorf("failed to create runtime event service for manifest controller: %w", err)
	}

	// Use interface for clean dependency management
	var eventService EventService = runtimeEventService

	// Note: Service is automatically started by the manager (implements manager.Runnable)

	// Create event handler for manifest events
	addRuntimeEventToQueueFunc := &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
			queue workqueue.TypedRateLimitingInterface[ctrl.Request],
		) {
			logger := ctrl.Log.WithName("manifest-listener")
			unstructWatcherEvt, conversionOk := evnt.Object.(*unstructured.Unstructured)
			if !conversionOk {
				logger.Error(nil, "error converting event object", "event", evnt.Object)
				return
			}

			// Again, ExtractOwnerKey tells us WHICH Manifest to reconcile
			ownerObjectKey, err := skrevent.ExtractOwnerKey(unstructWatcherEvt)
			if err != nil {
				logger.Error(err, "failed to extract owner key from runtime watcher event")
				return
			}

			logger.Info("manifest event from runtime-watcher, adding to queue", "objectKey", ownerObjectKey.String())
			queue.Add(ctrl.Request{NamespacedName: ownerObjectKey})
		},
	}

	// Create event source for this controller
	runtimeEventSource := eventService.CreateEventSource(addRuntimeEventToQueueFunc)

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Named(controllerName).
		Watches(&apicorev1.Secret{}, handler.Funcs{},
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{},
				predicate.LabelChangedPredicate{}))).
		WatchesRawSource(runtimeEventSource).
		WithOptions(opts).
		Complete(NewReconciler(mgr, requeueIntervals, manifestMetrics, mandatoryModulesMetrics,
			manifestClient, orphanDetectionClient, specResolver)); err != nil {
		return fmt.Errorf("failed to setup manager for manifest controller: %w", err)
	}

	return nil
}
