package modulereleasemeta

import (
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta/events"
)

type kymaRepository interface {
	LookupByLabel(ctx context.Context, labelKey, labelValue string) (*v1beta2.KymaList, error)
}

type EventHandler = TypedEventHandler[client.Object, reconcile.Request]

// TypedEventHandler implements handler.EventHandler for ModuleReleaseMeta objects.
type TypedEventHandler[object any, request comparable] struct {
	kymaRepository kymaRepository

	// updateRequeueMaxDelay is the upper bound for the random delay applied when requeueing Kymas on
	// ModuleReleaseMeta update events. Spreading reconciliations over this window avoids rate-limiting
	// bursts when a new module version is rolled out to many clusters simultaneously.
	updateRequeueMaxDelay time.Duration

	CreateFunc func(context.Context, event.TypedCreateEvent[object], workqueue.TypedRateLimitingInterface[request])

	UpdateFunc func(context.Context, event.TypedUpdateEvent[object], workqueue.TypedRateLimitingInterface[request])

	DeleteFunc func(context.Context, event.TypedDeleteEvent[object], workqueue.TypedRateLimitingInterface[request])

	GenericFunc func(context.Context, event.TypedGenericEvent[object], workqueue.TypedRateLimitingInterface[request])
}

// NewEventHandler creates a handler that, on update events, spreads Kyma reconciliations
// over a random delay of up to updateRequeueMaxDelay to avoid rate-limiting bursts.
func NewEventHandler(kymaRepository kymaRepository, updateRequeueMaxDelay time.Duration) *EventHandler {
	return &EventHandler{
		kymaRepository:        kymaRepository,
		updateRequeueMaxDelay: updateRequeueMaxDelay,
	}
}

// Create handles Create events.
func (m TypedEventHandler[object, request]) Create(_ context.Context, _ event.CreateEvent,
	_ workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// NOOP - if a new MRM is introduced, no Kyma is using it yet, hence no Kymas can be added to the queue.
}

// Generic handles Generic events.
func (m TypedEventHandler[object, request]) Generic(_ context.Context, _ event.GenericEvent,
	_ workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// NOOP - we don't want to react to Generic Events.
}

// Delete handles Delete events.
func (m TypedEventHandler[object, request]) Delete(ctx context.Context, evt event.DeleteEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	kymaList, err := m.kymaRepository.LookupByLabel(ctx, shared.ManagedBy, shared.OperatorName)
	if err != nil {
		return
	}
	events.HandleDelete(evt, rli, kymaList)
}

// Update handles Update events. Affected Kymas are requeued with a random delay in [0, updateRequeueMaxDelay]
// to spread reconciliations and avoid rate-limiting bursts when a new module version is released.
func (m TypedEventHandler[object, request]) Update(ctx context.Context, evt event.UpdateEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	kymaList, err := m.kymaRepository.LookupByLabel(ctx, shared.ManagedBy, shared.OperatorName)
	if err != nil {
		return
	}
	events.HandleUpdate(evt, rli, kymaList, m.updateRequeueMaxDelay)
}
