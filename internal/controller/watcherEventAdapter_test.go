package controller_test

import (
	"testing"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/v2/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/internal/controller"
)

type IncomingListenerEvent = controller.WatcherListenerEvent

func TestAdaptEvents(t *testing.T) {
	t.Parallel()

	t.Run("The event gets converted", func(t *testing.T) {
		t.Parallel()

		// given
		unstructuredEvtObject := watcherevent.GenericEvent(testObject())
		testEvent := IncomingListenerEvent{Object: unstructuredEvtObject}

		sourceChan := make(chan IncomingListenerEvent)

		adaptedChan := controller.AdaptEvents(func() <-chan IncomingListenerEvent {
			return sourceChan
		})

		// when
		sourceChan <- testEvent // notice there's no blocking on the channel, although it's unbuffered
		adaptedEvt := <-adaptedChan

		// then
		receivedObj := adaptedEvt.Object

		assert.NotNil(t, receivedObj)
		assert.Equal(t, testEvent.Object.GetName(), receivedObj.GetName())
		assert.Equal(t, testEvent.Object.GetNamespace(), receivedObj.GetNamespace())
	})

	t.Run("The adapted channel gets closed after the source closes", func(t *testing.T) {
		t.Parallel()

		// given
		unstructuredEvtObject := watcherevent.GenericEvent(testObject())
		testEvent := IncomingListenerEvent{Object: unstructuredEvtObject}

		sourceChan := make(chan IncomingListenerEvent)

		adaptedChan := controller.AdaptEvents(func() <-chan IncomingListenerEvent {
			return sourceChan
		})

		// when
		sourceChan <- testEvent // notice there's no blocking on the channel, although it's unbuffered
		adaptedEvt := <-adaptedChan

		// then
		assert.NotNil(t, adaptedEvt.Object)

		// and when
		close(sourceChan)
		_, ok := <-adaptedChan

		// then
		assert.False(t, ok)
	})
}

func testObject() *types.WatchEvent {
	// Taken from `runtime-watcher/listener/pkg/v2/event/watcher_event_test.go`
	return &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "kyma", Namespace: apimetav1.NamespaceDefault},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: "some-namespace"},
		WatchedGvk: apimetav1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
	}
}
