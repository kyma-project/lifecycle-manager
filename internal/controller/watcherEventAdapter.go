package controller

import (
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type (
	WatcherListenerEvent      = types.GenericEvent
	TypedGenericListenerEvent = event.TypedGenericEvent[types.GenericEvent]
)

// AdaptEvents converts given channel from the type used by runtime-watcher/listener
// module to the type required by the controller-runtime library.
func AdaptEvents(listenerChan func() <-chan WatcherListenerEvent) <-chan TypedGenericListenerEvent {
	dest := make(chan TypedGenericListenerEvent)
	go func() {
		defer close(dest)
		for evt := range listenerChan() {
			dest <- TypedGenericListenerEvent{
				Object: evt,
			}
		}
	}()
	return dest
}
