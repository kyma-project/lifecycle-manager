package event

import (
	"context"
	"fmt"
	"sync"

	watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kyma-project/lifecycle-manager/internal/common"
)

// Implement manager.Runnable interface for automatic lifecycle management.
var _ manager.Runnable = (*SkrRuntimeEventService)(nil)

// Implement common.EventService interface for dependency injection.
var _ common.EventService = (*SkrRuntimeEventService)(nil)

type SkrRuntimeEventService struct {
	listener     *watcherevent.SKREventListener
	eventChannel chan event.GenericEvent // Single channel - no pub/sub
	cancel       context.CancelFunc      // Only store cancel func, not context
	once         sync.Once               // Ensure channel is closed only once
	cancelMutex  sync.Mutex              // Protect cancel function access
}

const (
	// Default buffer size for the event channel.
	defaultEventChannelBuffer = 100
)

// NewSkrRuntimeEventService creates a simplified event service.
func NewSkrRuntimeEventService(listener *watcherevent.SKREventListener) *SkrRuntimeEventService {
	return &SkrRuntimeEventService{
		listener:     listener,
		eventChannel: make(chan event.GenericEvent, defaultEventChannelBuffer),
	}
}

// CreateEventSource returns the single event source.
//
//nolint:ireturn // Interface compliance required for controller-runtime integration.
func (s *SkrRuntimeEventService) CreateEventSource(handler handler.EventHandler) source.Source {
	return source.Channel(s.eventChannel, handler)
}

// Start begins simple event forwarding - implements manager.Runnable interface.
func (s *SkrRuntimeEventService) Start(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)

	// Thread-safe assignment of cancel function
	s.cancelMutex.Lock()
	s.cancel = cancel
	s.cancelMutex.Unlock()

	// Start event forwarding FIRST (before listener blocks)
	go s.forwardEvents(childCtx)

	// Start the listener with the context (if listener exists)
	// This will block until context is cancelled
	if s.listener != nil {
		err := s.listener.Start(childCtx)
		if err != nil {
			return fmt.Errorf("failed to start SKR event listener: %w", err)
		}
	}

	return nil
}

// Stop shuts down - managed automatically by controller-runtime manager
// Implements manager.Runnable interface.
func (s *SkrRuntimeEventService) Stop() error {
	// Thread-safe access to cancel function
	s.cancelMutex.Lock()
	defer s.cancelMutex.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	return nil
}

// forwardEvents - Direct 1:1 forwarding from listener to controller-runtime.
// This goroutine reads events from the listener's ReceivedEvents channel and forwards
// them to the eventChannel that controller-runtime uses as an event source.
func (s *SkrRuntimeEventService) forwardEvents(ctx context.Context) {
	// Ensure channel is closed only once using sync.Once
	defer s.once.Do(func() {
		close(s.eventChannel)
	})

	// If no listener, just wait for context cancellation
	if s.listener == nil {
		<-ctx.Done()
		return
	}

	for {
		select {
		case listenerEvent, ok := <-s.listener.ReceivedEvents:
			if !ok {
				return // Listener closed
			}

			// Convert from listener types.GenericEvent to controller-runtime event.GenericEvent
			crEvent := event.GenericEvent{
				Object: listenerEvent.Object,
			}

			// Forward to single channel (no distribution logic)
			select {
			case s.eventChannel <- crEvent:
				// Forwarded successfully
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
