package kyma

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

var (
	ErrHandlingWatcherEvent   = errors.New("error handling watcher event")
	ErrConvertingWatcherEvent = errors.New("failed to convert event object")
	ErrExtractingRuntimeID    = errors.New("failed to extract runtime ID from event data")
)

type KymaLookupService interface {
	NameByRuntimeID(ctx context.Context, runtimeID string) (string, error)
}

func CreateSkrEventHandler(kymaLookup KymaLookupService) *handler.Funcs {
	return &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
			queue workqueue.TypedRateLimitingInterface[ctrl.Request],
		) {
			logger := ctrl.Log.WithName("listener")
			runtimeID, err := GetRuntimeIDFromEvent(evnt)
			if err != nil {
				logger.Error(fmt.Errorf("%w: %w", ErrHandlingWatcherEvent, err), fmt.Sprintf("event: %v", evnt.Object))
				return
			}
			kcpKymaName, err := kymaLookup.NameByRuntimeID(ctx, runtimeID)
			if err != nil {
				logger.Error(fmt.Errorf("%w: %w", ErrHandlingWatcherEvent, err), fmt.Sprintf("event: %v", evnt.Object))
				return
			}

			kcpKymaKey := client.ObjectKey{
				Name:      kcpKymaName,
				Namespace: shared.DefaultControlPlaneNamespace,
			}
			req := ctrl.Request{NamespacedName: kcpKymaKey}
			logger.Info(fmt.Sprintf("event received from SKR, adding %s to queue", req.NamespacedName))

			queue.Add(req)
		},
	}
}

func GetRuntimeIDFromEvent(evnt event.GenericEvent) (string, error) {
	unstruct, ok := evnt.Object.(*unstructured.Unstructured)
	if !ok {
		return "", ErrConvertingWatcherEvent
	}
	runtimeID, ok := ExtractRuntimeIDFromMap(unstruct)
	if !ok {
		return "", ErrExtractingRuntimeID
	}
	return runtimeID, nil
}

func ExtractRuntimeIDFromMap(unstructuredEvent *unstructured.Unstructured) (string, bool) {
	runtimeId, ok := unstructuredEvent.Object["runtime-id"]
	if !ok {
		return "", false
	}
	s, ok := runtimeId.(string)
	if !ok {
		return "", false
	}
	return s, true
}
