package kyma

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var (
	errParsingRuntimeID = errors.New("error getting runtime id from unstructured event")
)

type SkrEventHandler struct{}

func NewSkrEventHandler() *SkrEventHandler {
	return &SkrEventHandler{}
}

func (s *SkrEventHandler) Create(_ context.Context,
	_ event.TypedCreateEvent[types.GenericEvent],
	_ workqueue.TypedRateLimitingInterface[ctrl.Request],
) {
	// SkrEventHandler only handles Generic events
}

func (s *SkrEventHandler) Update(_ context.Context,
	_ event.TypedUpdateEvent[types.GenericEvent],
	_ workqueue.TypedRateLimitingInterface[ctrl.Request],
) {
	// SkrEventHandler only handles Generic events
}

func (s *SkrEventHandler) Delete(_ context.Context,
	_ event.TypedDeleteEvent[types.GenericEvent],
	_ workqueue.TypedRateLimitingInterface[ctrl.Request],
) {
	// SkrEventHandler only handles Generic events
}

func (s *SkrEventHandler) Generic(_ context.Context,
	evnt event.TypedGenericEvent[types.GenericEvent],
	queue workqueue.TypedRateLimitingInterface[ctrl.Request],
) {
	logger := ctrl.Log.WithName("listener")
	unstructWatcherEvt := evnt.Object.Object

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
}
