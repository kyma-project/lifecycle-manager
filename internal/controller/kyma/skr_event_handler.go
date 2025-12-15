package kyma

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

func CreateSkrEventHandler() *handler.Funcs {
	return &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
			queue workqueue.TypedRateLimitingInterface[ctrl.Request],
		) {
			logger := ctrl.Log.WithName("listener")
			req, ok := BuildRequestFromEvent(evnt)
			if !ok {
				logger.Error(errConvertingWatcherEvent, fmt.Sprintf("event: %v", evnt.Object))
				fmt.Printf( //nolint:forbidigo // debug line
					"===[DEBUG]====> %s: event: %v\n",
					errConvertingWatcherEvent, evnt.Object)
				return
			}
			logger.Info(fmt.Sprintf("event received from SKR, adding %s to queue", req.NamespacedName))
			fmt.Printf( //nolint:forbidigo // debug line
				"===[DEBUG]====> event received from SKR, adding %s to queue",
				req.NamespacedName)
			queue.Add(req)
		},
	}
}

func BuildRequestFromEvent(evnt event.GenericEvent) (ctrl.Request, bool) {
	unstruct, ok := evnt.Object.(*unstructured.Unstructured)
	if !ok {
		return ctrl.Request{}, false
	}
	runtimeID, ok := GetRuntimeID(unstruct)
	if !ok {
		return ctrl.Request{}, false
	}
	ownerObjectKey := client.ObjectKey{
		Name:      runtimeID,
		Namespace: shared.DefaultControlPlaneNamespace,
	}
	return ctrl.Request{NamespacedName: ownerObjectKey}, true
}

func GetRuntimeID(unstructuredEvent *unstructured.Unstructured) (string, bool) {
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
