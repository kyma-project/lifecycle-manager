package listener

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SKREventsListener struct {
	Addr   string
	Logger logr.Logger
	receivedEvents chan event.GenericEvent
}

const paramContractVersion = "contractVersion"

func (l *SKREventsListener) Start(ctx context.Context) error {
	//routing
	mainRouter := mux.NewRouter()
	apiRouter := mainRouter.PathPrefix("/").Subrouter()

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/listener", paramContractVersion),
		transformWatcherEvents(l.receivedEvents)).
		Methods(http.MethodPost)

	//start web server
	server := &http.Server{Addr: l.Addr, Handler: mainRouter}
	go func() {
		var err error
		err = server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			l.Logger.Error(err, "Webserver startup failed")
		}
	}()
	<-ctx.Done()
	l.Logger.Info("SKR events listener is shutting down: context got closed")
	return server.Shutdown(ctx)
}

func transformWatcherEvents(c chan event.GenericEvent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//TODO: put events to the channel
	}
}

func (l *SKREventsListener) ReceivedEvents() chan event.GenericEvent {
	if l.receivedEvents == nil {
		l.receivedEvents = make(chan event.GenericEvent)
	}
	return l.receivedEvents
}

type WatcherEventsHandler struct {
	client.Reader
	client.StatusWriter
	record.EventRecorder
}

func (h *WatcherEventsHandler) ProcessWatcherEvent(ctx context.Context) func(event.GenericEvent, workqueue.RateLimitingInterface) {
	logger := log.FromContext(ctx).WithName("skr-watcher-events-processing")
	return func(genericEvent event.GenericEvent, q workqueue.RateLimitingInterface) {
		//TODO: annotate component with the event gotten from the SKR watcher
		logger.Info("started dispatching event", genericEvent)
	}
}
