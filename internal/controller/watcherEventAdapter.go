package controller

import (
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type (
	WatcherListenerEvent = types.GenericEvent
	CtrlRuntimeEvent     = event.GenericEvent
)

// adaptEvents converts given channel from the type used by runtime-watcher/listener module to the type required by the controller-runtime library.
func AdaptEvents(listenerChan func() <-chan WatcherListenerEvent) <-chan CtrlRuntimeEvent {
	dest := make(chan event.GenericEvent)
	go func() {
		defer close(dest)
		for e := range listenerChan() {
			dest <- event.GenericEvent{Object: e.Object}
		}
	}()
	return dest
}
