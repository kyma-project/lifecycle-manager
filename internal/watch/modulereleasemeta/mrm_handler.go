package modulereleasemeta

import (
	"context"
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/types"
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
	mrm, ok := evt.Object.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}

	kymaList, err := m.kymaRepository.LookupByLabel(ctx, shared.ManagedBy, shared.OperatorName)
	if err != nil {
		return
	}

	requeueKymas(rli, events.AffectedKymasOnDelete(mrm, kymaList))
}

// Update handles Update events. Affected Kymas are requeued with a random delay in [0, updateRequeueMaxDelay)
// to spread reconciliations and avoid rate-limiting bursts when a new module version is released.
func (m TypedEventHandler[object, request]) Update(ctx context.Context, evt event.UpdateEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	oldMRM, ok := evt.ObjectOld.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}
	newMRM, ok := evt.ObjectNew.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}

	kymaList, err := m.kymaRepository.LookupByLabel(ctx, shared.ManagedBy, shared.OperatorName)
	if err != nil {
		return
	}

	requeueKymasWithRandomDelay(rli, events.AffectedKymasOnUpdate(oldMRM, newMRM, kymaList), m.updateRequeueMaxDelay)
}

func requeueKymas(rli workqueue.TypedRateLimitingInterface[reconcile.Request], kymas []*types.NamespacedName) {
	for _, kyma := range kymas {
		rli.Add(reconcile.Request{NamespacedName: *kyma})
	}
}

// requeueKymasWithRandomDelay enqueues each Kyma with a uniformly random delay in [0, maxDelay].
// When maxDelay is zero the items are added immediately (same behaviour as requeueKymas).
func requeueKymasWithRandomDelay(
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
	kymas []*types.NamespacedName,
	maxDelay time.Duration,
) {
	for _, kyma := range kymas {
		req := reconcile.Request{NamespacedName: *kyma}
		if maxDelay <= 0 {
			rli.Add(req)
			continue
		}
		delay := time.Duration(rand.Int63n(int64(maxDelay))) //nolint:gosec // non-cryptographic jitter
		rli.AddAfter(req, delay)
	}
}
