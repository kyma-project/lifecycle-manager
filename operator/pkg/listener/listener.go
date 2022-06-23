package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type UnmarshalError struct {
	Message       string
	httpErrorCode int
}

type WatcherEvent struct {
	SkrClusterID string `json:"skrClusterID"`
	Namespace    string `json:"namespace"`
	ResourceName string `json:"resourceName"`
	Component    string `json:"component"`
}

type SKREventsListener struct {
	Addr           string
	Logger         logr.Logger
	receivedEvents chan event.GenericEvent
}

const paramContractVersion = "contractVersion"

func (l *SKREventsListener) ReceivedEvents() chan event.GenericEvent {
	if l.receivedEvents == nil {
		l.receivedEvents = make(chan event.GenericEvent)
	}
	return l.receivedEvents
}

func (l *SKREventsListener) Start(ctx context.Context) error {
	//routing
	mainRouter := mux.NewRouter()
	apiRouter := mainRouter.PathPrefix("/").Subrouter()

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/skr/events/create", paramContractVersion),
		l.handleCreateEvent()).
		Methods(http.MethodPost)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/skr/events/update", paramContractVersion),
		l.handleUpdateEvent()).
		Methods(http.MethodPost)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/skr/events/delete", paramContractVersion),
		l.handleDeleteEvent()).
		Methods(http.MethodPost)

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/skr/events/generic", paramContractVersion),
		l.handleGenericEvent()).
		Methods(http.MethodPost)

	//start web server
	server := &http.Server{Addr: l.Addr, Handler: mainRouter}
	go func() {
		l.Logger.Info("SKR events listener is starting up...")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			l.Logger.Error(err, "Webserver startup failed")
		}
		l.Logger.WithValues("Address:", server.Addr).
			Info("SKR events listener started up successfully")
	}()
	<-ctx.Done()
	l.Logger.Info("SKR events listener is shutting down: context got closed")
	return server.Shutdown(ctx)
}

func (l *SKREventsListener) handleCreateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l.Logger.Info("CreateEvent")

		//unmarshal received event
		genEvtObject, unmarshalErr := unmarshalEvent(r)
		if unmarshalErr != nil {
			w.WriteHeader(unmarshalErr.httpErrorCode)
			w.Write([]byte(unmarshalErr.Message))
			return
		}

		//add event to the channel
		l.receivedEvents <- event.GenericEvent{Object: genEvtObject}
		w.WriteHeader(http.StatusOK)
	}
}

func (l *SKREventsListener) handleUpdateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l.Logger.Info("UpdateEvent")

		//unmarshal received event
		genEvtObject, unmarshalErr := unmarshalEvent(r)
		if unmarshalErr != nil {
			w.WriteHeader(unmarshalErr.httpErrorCode)
			w.Write([]byte(unmarshalErr.Message))
			return
		}

		//add event to the channel
		l.receivedEvents <- event.GenericEvent{Object: genEvtObject}
		w.WriteHeader(http.StatusOK)
	}
}

func (l *SKREventsListener) handleDeleteEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l.Logger.Info("DeleteEvent")
		w.WriteHeader(http.StatusOK)
	}
}

func (l *SKREventsListener) handleGenericEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l.Logger.Info("GenericEvent")
		w.WriteHeader(http.StatusOK)
	}
}

func unmarshalEvent(r *http.Request) (*unstructured.Unstructured, *UnmarshalError) {
	params := mux.Vars(r)
	contractVersion, ok := params[paramContractVersion]
	if !ok {
		return nil, &UnmarshalError{"contract version could not be parsed", http.StatusBadRequest}
	}

	if contractVersion == "" {
		return nil, &UnmarshalError{"contract version cannot be empty", http.StatusBadRequest}
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, &UnmarshalError{"could not read request body", http.StatusInternalServerError}
	}

	watcherEvent := &WatcherEvent{}
	err = json.Unmarshal(body, watcherEvent)
	if err != nil {
		return nil, &UnmarshalError{"could not unmarshal watcher event", http.StatusInternalServerError}
	}

	genEvtObject := &unstructured.Unstructured{}
	genEvtObject.SetName(watcherEvent.ResourceName)
	genEvtObject.SetClusterName(watcherEvent.SkrClusterID)
	genEvtObject.SetNamespace(watcherEvent.Namespace)

	return genEvtObject, nil
}
