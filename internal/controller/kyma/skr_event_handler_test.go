package kyma_test

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma"
)

func TestExtractRuntimeIDFromMap_Valid(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"runtime-id": "rid-123"}
	rid, ok := kyma.ExtractRuntimeIDFromMap(unstructuredEvent)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if rid != "rid-123" {
		t.Fatalf("expected rid-123, got %s", rid)
	}
}

func TestExtractRuntimeIDFromMap_Missing(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"other": "x"}
	_, ok := kyma.ExtractRuntimeIDFromMap(unstructuredEvent)
	if ok {
		t.Fatalf("expected ok=false when runtime-id missing")
	}
}

func TestExtractRuntimeIDFromMap_WrongType(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"runtime-id": 123}
	_, ok := kyma.ExtractRuntimeIDFromMap(unstructuredEvent)
	if ok {
		t.Fatalf("expected ok=false when runtime-id not string")
	}
}

func TestGetRuntimeIDFromEvent_Valid(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"runtime-id": "rid-456"}
	runtimeID, err := kyma.GetRuntimeIDFromEvent(event.GenericEvent{Object: unstructuredEvent})
	if err != nil {
		t.Fatalf("expected no error for a valid event")
	}
	if runtimeID != "rid-456" {
		t.Fatalf("expected name rid-456, got %s", runtimeID)
	}
}

func TestGetRuntimeIDFromEvent_NotUnstructured(t *testing.T) {
	// Object is nil, which does not satisfy *unstructured.Unstructured
	_, err := kyma.GetRuntimeIDFromEvent(event.GenericEvent{Object: nil})
	if !errors.Is(err, kyma.ErrConvertingWatcherEvent) {
		t.Fatalf("expected error when event object is not unstructured")
	}
}

func TestGetRuntimeIDFromEvent_MissingRuntimeID(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"other": "x"}
	_, err := kyma.GetRuntimeIDFromEvent(event.GenericEvent{Object: unstructuredEvent})
	if !errors.Is(err, kyma.ErrExtractingRuntimeID) {
		t.Fatalf("expected error when runtime-id missing")
	}
}

func TestGetRuntimeIDFromEvent_RuntimeIDWrongType(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"runtime-id": 123}
	_, err := kyma.GetRuntimeIDFromEvent(event.GenericEvent{Object: unstructuredEvent})
	if !errors.Is(err, kyma.ErrExtractingRuntimeID) {
		t.Fatalf("expected error when runtime-id has a wrong type")
	}
}

func TestSkrEventHandler_GenericFunc_AddsToQueue(t *testing.T) {
	handler := kyma.CreateSkrEventHandler(&mockKymaLookup{"kyma-789"})
	rl := internal.RateLimiter(100, 1000, 10, 100)
	queue := workqueue.NewTypedRateLimitingQueue[ctrl.Request](rl)
	defer queue.ShutDown()

	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"runtime-id": "rid-789"}
	ev := event.GenericEvent{Object: unstructuredEvent}

	// Call GenericFunc
	handler.GenericFunc(context.Background(), ev, queue)

	if queue.Len() != 1 {
		t.Fatalf("expected queue length 1, got %d", queue.Len())
	}
	// Verify the enqueued request matches expected NamespacedName
	item, shutdown := queue.Get()
	if shutdown {
		t.Fatalf("unexpected queue shutdown")
	}
	queue.Done(item)
	req := item
	if req.Name != "kyma-789" {
		t.Fatalf("expected name kyma-789, got %s", req.Name)
	}
	if req.Namespace != shared.DefaultControlPlaneNamespace {
		t.Fatalf("expected namespace %s, got %s", shared.DefaultControlPlaneNamespace, req.Namespace)
	}
}

func TestSkrEventHandler_GenericFunc_ResolverError_NoAdd(t *testing.T) {
	handler := kyma.CreateSkrEventHandler(&errorKymaLookup{})
	rl := internal.RateLimiter(100, 1000, 10, 100)
	queue := workqueue.NewTypedRateLimitingQueue[ctrl.Request](rl)
	defer queue.ShutDown()

	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"runtime-id": "rid-789"}
	ev := event.GenericEvent{Object: unstructuredEvent}

	// Call GenericFunc
	handler.GenericFunc(context.Background(), ev, queue)

	if queue.Len() != 0 {
		t.Fatalf("expected queue length 0, got %d", queue.Len())
	}
}

func TestSkrEventHandler_GenericFunc_InvalidEvent_NoAdd(t *testing.T) {
	handler := kyma.CreateSkrEventHandler(&mockKymaLookup{"kyma-789"})
	rl := internal.RateLimiter(100, 1000, 10, 100)
	queue := workqueue.NewTypedRateLimitingQueue[ctrl.Request](rl)
	defer queue.ShutDown()

	// invalid: not unstructured (nil object)
	ev := event.GenericEvent{Object: nil}
	handler.GenericFunc(context.Background(), ev, queue)

	if queue.Len() != 0 {
		t.Fatalf("expected queue length 0 for invalid event, got %d", queue.Len())
	}

	// invalid: unstructured without runtime-id
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]any{"other": "x"}
	ev2 := event.GenericEvent{Object: unstructuredEvent}
	handler.GenericFunc(context.Background(), ev2, queue)

	if queue.Len() != 0 {
		t.Fatalf("expected queue length 0 when runtime-id missing, got %d", queue.Len())
	}
}

type mockKymaLookup struct {
	expectedKymaName string
}

func (m *mockKymaLookup) NameByRuntimeID(ctx context.Context, runtimeID string) (string, error) {
	return m.expectedKymaName, nil
}

type errorKymaLookup struct{}

func (e *errorKymaLookup) NameByRuntimeID(ctx context.Context, runtimeID string) (string, error) {
	return "", errors.New("mock resolver error")
}
