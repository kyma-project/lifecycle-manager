package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// WatcherEvent TODO: update Watcher Event fields
type WatcherEvent struct {
	SkrClusterID string `json:"skrClusterID"`
	Body         []byte `json:"body"`
	EventType    string `json:"eventType"`
}

type GenericEventObject struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

// DeepCopyObject TODO: use kube builder to generate this
func (g *GenericEventObject) DeepCopyObject() runtime.Object {
	if c := g.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy TODO: use kube builder to generate this
func (g *GenericEventObject) DeepCopy() *GenericEventObject {

	if g == nil {
		return nil
	}
	ng := new(GenericEventObject)
	*ng = *g
	ng.TypeMeta = g.TypeMeta
	g.ObjectMeta.DeepCopyInto(&ng.ObjectMeta)
	return ng
}

type SKREventsListener struct {
	Addr           string
	Logger         logr.Logger
	receivedEvents chan event.GenericEvent
}

const paramContractVersion = "contractVersion"

func (l *SKREventsListener) Start(ctx context.Context) error {
	//TODO: replace gorilla mux path routing with vanilla path routing
	//routing
	mainRouter := mux.NewRouter()
	apiRouter := mainRouter.PathPrefix("/").Subrouter()

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/skr/events", paramContractVersion),
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
		l.Logger.WithValues("Address:", server.Addr).
			Info("SKR events listener started up successfully")
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
		l.Logger.Info(fmt.Sprintf("B Body: %s", b))

		if contractVersion == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("contract version cannot be empty"))
			return
		}

		watcherEvent := &WatcherEvent{}
		err = json.Unmarshal(b, watcherEvent)
		if err != nil {
			l.Logger.Info(fmt.Sprintf("ERROR  while unmarshaling%s", err))
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("could not unmarshal watcher event"))
			return
		}
		l.Logger.Info(fmt.Sprintf("WatcherEvent: %#v", watcherEvent))

		l.Logger.Info(fmt.Sprintf("EventType: %s", watcherEvent.EventType))
		var componentName string
		l.Logger.Info(fmt.Sprintf("Body: %s", watcherEvent.Body))

		l.Logger.Info(fmt.Sprintf("Component Name: %s", componentName))
		switch watcherEvent.EventType {
		case "create":
			l.Logger.Info("CreateEvent")
			//component := unstructured.Unstructured{}
			//if err = json.Unmarshal(watcherEvent.Body, &component); err != nil {
			//	l.Logger.Error(err, "error transforming new component object")
			//	return
			//}
			//l.Logger.Info(fmt.Sprintf("Component after CreateEvent: %#v", component))

			//objectBytesNew, err := json.Marshal(watcherEvent.Body)
			//if err != nil {
			//	l.Logger.Error(err, "error transforming new component object")
			//	return
			//}

			component := unstructured.Unstructured{}
			if err = json.Unmarshal(watcherEvent.Body, &component); err != nil {
				l.Logger.Error(err, "error transforming new component object")
				return
			}

			//componentName = component.GetName()
		case "update":
			l.Logger.Info("UpdateEvent")
			//TODO: compare names of new object and old object (if resource name is used for mapping)
			//componentName = watcherEvent.Body.(event.UpdateEvent).ObjectNew.GetName()
		case "delete":
			l.Logger.Info("DeleteEvent")
			//TODO: check DeleteStateUnknown
			//componentName = watcherEvent.Body.(event.DeleteEvent).Object.GetName()
		case "generic":
			l.Logger.Info("GenericEvent")
			//componentName = watcherEvent.Body.(event.GenericEvent).Object.GetName()
		default:
			l.Logger.Info("Default Case - Should not happen")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("could not unmarshal watcher event body"))
			return
		}

		//add event to the channel
		genericEvtObject := &GenericEventObject{}
		genericEvtObject.SetName(componentName)
		genericEvtObject.SetClusterName(watcherEvent.SkrClusterID)
		l.receivedEvents <- event.GenericEvent{Object: genericEvtObject}
		w.WriteHeader(http.StatusOK)
	}
}

func (l *SKREventsListener) ReceivedEvents() chan event.GenericEvent {
	if l.receivedEvents == nil {
		l.receivedEvents = make(chan event.GenericEvent)
	}
	return l.receivedEvents
}
