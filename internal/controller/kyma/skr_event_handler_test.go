package kyma_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma"
)

func TestGetRuntimeID_Valid(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"runtime-id": "rid-123"}
	rid, ok := kyma.GetRuntimeID(unstructuredEvent)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if rid != "rid-123" {
		t.Fatalf("expected rid-123, got %s", rid)
	}
}

func TestGetRuntimeID_Missing(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"other": "x"}
	_, ok := kyma.GetRuntimeID(unstructuredEvent)
	if ok {
		t.Fatalf("expected ok=false when runtime-id missing")
	}
}

func TestGetRuntimeID_WrongType(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"runtime-id": 123}
	_, ok := kyma.GetRuntimeID(unstructuredEvent)
	if ok {
		t.Fatalf("expected ok=false when runtime-id not string")
	}
}

func TestBuildRequestFromEvent_Valid(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"runtime-id": "rid-456"}
	req, ok := kyma.BuildRequestFromEvent(event.GenericEvent{Object: unstructuredEvent})
	if !ok {
		t.Fatalf("expected ok=true for valid event")
	}
	if req.Name != "rid-456" {
		t.Fatalf("expected name rid-456, got %s", req.Name)
	}
	if req.Namespace != shared.DefaultControlPlaneNamespace {
		t.Fatalf("expected namespace %s, got %s", shared.DefaultControlPlaneNamespace, req.Namespace)
	}
}

func TestBuildRequestFromEvent_NotUnstructured(t *testing.T) {
	// Object is nil, which does not satisfy *unstructured.Unstructured
	req, ok := kyma.BuildRequestFromEvent(event.GenericEvent{Object: nil})
	if ok {
		t.Fatalf("expected ok=false when event object is not unstructured")
	}
	if (req != ctrl.Request{}) {
		t.Fatalf("expected zero request when not unstructured")
	}
}

func TestBuildRequestFromEvent_MissingRuntimeID(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"other": "x"}
	req, ok := kyma.BuildRequestFromEvent(event.GenericEvent{Object: unstructuredEvent})
	if ok {
		t.Fatalf("expected ok=false when runtime-id missing")
	}
	if (req != ctrl.Request{}) {
		t.Fatalf("expected zero request when runtime-id missing")
	}
}

func TestBuildRequestFromEvent_RuntimeIDWrongType(t *testing.T) {
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"runtime-id": 123}
	req, ok := kyma.BuildRequestFromEvent(event.GenericEvent{Object: unstructuredEvent})
	if ok {
		t.Fatalf("expected ok=false when runtime-id not string")
	}
	if (req != ctrl.Request{}) {
		t.Fatalf("expected zero request when runtime-id wrong type")
	}
}

func TestSkrEventHandler_GenericFunc_AddsToQueue(t *testing.T) {
	handler := kyma.CreateSkrEventHandler()
	rl := internal.RateLimiter(100, 1000, 10, 100)
	queue := workqueue.NewTypedRateLimitingQueue[ctrl.Request](rl)
	defer queue.ShutDown()

	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"runtime-id": "rid-789"}
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
	if req.Name != "rid-789" {
		t.Fatalf("expected name rid-789, got %s", req.Name)
	}
	if req.Namespace != shared.DefaultControlPlaneNamespace {
		t.Fatalf("expected namespace %s, got %s", shared.DefaultControlPlaneNamespace, req.Namespace)
	}
}

func TestSkrEventHandler_GenericFunc_InvalidEvent_NoAdd(t *testing.T) {
	handler := kyma.CreateSkrEventHandler()
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
	unstructuredEvent.Object = map[string]interface{}{"other": "x"}
	ev2 := event.GenericEvent{Object: unstructuredEvent}
	handler.GenericFunc(context.Background(), ev2, queue)

	if queue.Len() != 0 {
		t.Fatalf("expected queue length 0 when runtime-id missing, got %d", queue.Len())
	}
}
