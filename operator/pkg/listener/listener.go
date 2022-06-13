package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// WatcherEvent TODO: update Watcher Event fields
type WatcherEvent struct {
	SkrClusterID  string `json:"skrClusterID"`
	Type          string `json:"eventType"`
	ComponentName string `json:"componentName"`
	ConfigData    string `json:"configData"`
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
		genericEvtObject := &GenericEventObject{}
		genericEvtObject.SetName(watcherEvent.ComponentName)
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
