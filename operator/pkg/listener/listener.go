package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type EventType string

type WatcherEvent struct {
	SkrClusterID string             `json:"skrClusterID"`
	Type         EventType          `json:"eventType"`
	Body         event.GenericEvent `json:"eventBody"`
}

type SKREventsListener struct {
	Addr           string
	Logger         logr.Logger
	receivedEvents chan event.GenericEvent
}

const paramContractVersion = "contractVersion"

func (l *SKREventsListener) Start(ctx context.Context) error {
	//routing
	mainRouter := mux.NewRouter()
	apiRouter := mainRouter.PathPrefix("/").Subrouter()

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/listener", paramContractVersion),
		l.transformWatcherEvents()).
		Methods(http.MethodPost)

	//start web server
	server := &http.Server{Addr: l.Addr, Handler: mainRouter}
	go func() {
		l.Logger.Info("SKR events listener is starting up...")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			l.Logger.Error(err, "Webserver startup failed")
		}
	}()
	<-ctx.Done()
	l.Logger.Info("SKR events listener is shutting down: context got closed")
	return server.Shutdown(ctx)
}

func (l *SKREventsListener) transformWatcherEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		contractVersion, ok := params[paramContractVersion]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("contract version could not be parsed"))
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("could not read request body"))
			return
		}

		if contractVersion == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("contract version cannot be empty"))
			return
		}

		watcherEvent := &WatcherEvent{}
		err = json.Unmarshal(b, watcherEvent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("could not unmarshal watcher event"))
			return
		}

		//add event to the channel
		l.receivedEvents <- watcherEvent.Body
		w.WriteHeader(http.StatusOK)
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
}

func (h *WatcherEventsHandler) ProcessWatcherEvent(ctx context.Context) func(event.GenericEvent, workqueue.RateLimitingInterface) {
	logger := log.FromContext(ctx).WithName("skr-watcher-events-processing")
	return func(genericEvent event.GenericEvent, _ workqueue.RateLimitingInterface) {
		//Label component template with a current timestamp when an event is received from the SKR watcher
		logger.Info("started dispatching event", genericEvent)

		componentTemplate := &v1alpha1.ModuleTemplate{}
		eventObject := genericEvent.Object
		namespacedName := client.ObjectKeyFromObject(eventObject)

		if err := h.Get(ctx, namespacedName, componentTemplate); err != nil {
			logger.WithValues(
				"component", namespacedName.Name,
				"namespace", namespacedName.Namespace,
			).Error(err, "could not get component CR")
			return
		}
		componentLabels := componentTemplate.GetLabels()
		componentLabels[labels.ListenerLastUpdated] = time.Now().UTC().Format(time.RFC1123)
		componentTemplate.SetLabels(componentLabels)
		if err := h.Update(ctx, componentTemplate); err != nil {
			logger.WithValues(
				"component", namespacedName.Name,
				"namespace", namespacedName.Namespace,
			).Error(err, "could not update component CR to trigger the reconciliation loop")
			return
		}
		logger.WithValues(
			"component", namespacedName.Name,
			"namespace", namespacedName.Namespace,
		).Info("successfully update component CR to trigger its reconciliation loop")
	}
}
