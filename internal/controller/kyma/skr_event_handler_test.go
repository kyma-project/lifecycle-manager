package kyma_test

import (
	"context"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	internalpkg "github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
	if req.NamespacedName.Name != "rid-456" {
		t.Fatalf("expected name rid-456, got %s", req.NamespacedName.Name)
	}
	if req.NamespacedName.Namespace != shared.DefaultControlPlaneNamespace {
		t.Fatalf("expected namespace %s, got %s", shared.DefaultControlPlaneNamespace, req.NamespacedName.Namespace)
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
	h := kyma.SkrEventHandler()
	rl := internalpkg.RateLimiter(100, 1000, 10, 100)
	q := workqueue.NewTypedRateLimitingQueue[ctrl.Request](rl)
	defer q.ShutDown()

	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"runtime-id": "rid-789"}
	ev := event.GenericEvent{Object: unstructuredEvent}

	// Call GenericFunc
	h.GenericFunc(context.Background(), ev, q)

	if q.Len() != 1 {
		t.Fatalf("expected queue length 1, got %d", q.Len())
	}
	// Verify the enqueued request matches expected NamespacedName
	item, shutdown := q.Get()
	if shutdown {
		t.Fatalf("unexpected queue shutdown")
	}
	q.Done(item)
	req := item
	if req.NamespacedName.Name != "rid-789" {
		t.Fatalf("expected name rid-789, got %s", req.NamespacedName.Name)
	}
	if req.NamespacedName.Namespace != shared.DefaultControlPlaneNamespace {
		t.Fatalf("expected namespace %s, got %s", shared.DefaultControlPlaneNamespace, req.NamespacedName.Namespace)
	}
}

func TestSkrEventHandler_GenericFunc_InvalidEvent_NoAdd(t *testing.T) {
	h := kyma.SkrEventHandler()
	rl := internalpkg.RateLimiter(100, 1000, 10, 100)
	q := workqueue.NewTypedRateLimitingQueue[ctrl.Request](rl)
	defer q.ShutDown()

	// invalid: not unstructured (nil object)
	ev := event.GenericEvent{Object: nil}
	h.GenericFunc(context.Background(), ev, q)

	if q.Len() != 0 {
		t.Fatalf("expected queue length 0 for invalid event, got %d", q.Len())
	}

	// invalid: unstructured without runtime-id
	unstructuredEvent := &unstructured.Unstructured{}
	unstructuredEvent.Object = map[string]interface{}{"other": "x"}
	ev2 := event.GenericEvent{Object: unstructuredEvent}
	h.GenericFunc(context.Background(), ev2, q)

	if q.Len() != 0 {
		t.Fatalf("expected queue length 0 when runtime-id missing, got %d", q.Len())
	}
}
