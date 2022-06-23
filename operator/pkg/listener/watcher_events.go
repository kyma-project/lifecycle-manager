package listener

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type WatcherEventsHandler struct {
	client.Client
}

func (h *WatcherEventsHandler) ProcessWatcherEvent(ctx context.Context) func(event.GenericEvent, workqueue.RateLimitingInterface) {
	logger := log.FromContext(ctx).WithName("skr-watcher-events-processing")
	return func(genEvt event.GenericEvent, q workqueue.RateLimitingInterface) {

		kymaObjectKey := client.ObjectKeyFromObject(genEvt.Object)
		logger.WithValues(
			"object", kymaObjectKey.Name,
			"namespace", kymaObjectKey.Namespace,
		).Info("started dispatching SKR event")

		//Enqueue for object received from SKR
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      kymaObjectKey.Name,
			Namespace: kymaObjectKey.Namespace,
		}})

		logger.WithValues(
			"object", kymaObjectKey.Name,
			"namespace", kymaObjectKey.Namespace,
		).Info("finished dispatching SKR event")
	}
}
