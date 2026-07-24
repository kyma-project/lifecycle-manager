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
)

type EventHandler = TypedEventHandler[client.Object, reconcile.Request]

// TypedEventHandler implements handler.EventHandler for ModuleReleaseMeta objects. The resolver
// decides which Kymas are affected; on update events those Kymas are requeued with a random delay
// in [0, updateRequeueMaxDelay) to spread reconciliations and avoid rate-limiting bursts when a new
// module version is rolled out to many clusters simultaneously.
type TypedEventHandler[object any, request comparable] struct {
	kymaRepository        kymaRepository
	resolver              affectedKymasResolver
	updateRequeueMaxDelay time.Duration
}

func NewEventHandler(kymaRepository kymaRepository, resolver affectedKymasResolver,
	updateRequeueMaxDelay time.Duration,
) *EventHandler {
	return &EventHandler{
		kymaRepository:        kymaRepository,
		resolver:              resolver,
		updateRequeueMaxDelay: updateRequeueMaxDelay,
	}
}

func (m TypedEventHandler[object, request]) Create(ctx context.Context, evt event.CreateEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	mrm, kymas, ok := m.lookup(ctx, evt.Object)
	if !ok {
		return
	}
	requeueKymas(rli, m.resolver.OnCreate(mrm, kymas))
}

func (m TypedEventHandler[object, request]) Delete(ctx context.Context, evt event.DeleteEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	mrm, kymas, ok := m.lookup(ctx, evt.Object)
	if !ok {
		return
	}
	requeueKymas(rli, m.resolver.OnDelete(mrm, kymas))
}

// Update requeues affected Kymas with a random delay in [0, updateRequeueMaxDelay) to spread
// reconciliations and avoid rate-limiting bursts when a new module version is released.
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

	kymas, err := m.kymaRepository.LookupByLabel(ctx, shared.ManagedBy, shared.OperatorName)
	if err != nil {
		return
	}

	requeueKymasWithRandomDelay(rli, m.resolver.OnUpdate(oldMRM, newMRM, kymas), m.updateRequeueMaxDelay)
}

func (m TypedEventHandler[object, request]) Generic(_ context.Context, _ event.GenericEvent,
	_ workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// NOOP - we don't want to react to Generic Events.
}

func (m TypedEventHandler[object, request]) lookup(ctx context.Context,
	obj client.Object,
) (*v1beta2.ModuleReleaseMeta, *v1beta2.KymaList, bool) {
	mrm, ok := obj.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return nil, nil, false
	}

	kymas, err := m.kymaRepository.LookupByLabel(ctx, shared.ManagedBy, shared.OperatorName)
	if err != nil {
		return nil, nil, false
	}

	return mrm, kymas, true
}

func requeueKymas(rli workqueue.TypedRateLimitingInterface[reconcile.Request], kymas []*types.NamespacedName) {
	for _, kyma := range kymas {
		rli.Add(reconcile.Request{NamespacedName: *kyma})
	}
}

// requeueKymasWithRandomDelay enqueues each Kyma with a uniformly random delay in [0, maxDelay).
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
