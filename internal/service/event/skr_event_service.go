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
}

// NewSkrRuntimeEventService creates a simplified event service.
func NewSkrRuntimeEventService(listener *watcherevent.SKREventListener) *SkrRuntimeEventService {
	return &SkrRuntimeEventService{
		listener:     listener,
		eventChannel: make(chan event.GenericEvent, 100), // Single buffered channel
	}
}

// CreateEventSource returns the single event source.
func (s *SkrRuntimeEventService) CreateEventSource(handler handler.EventHandler) source.Source {
	// Simple: just return the single channel
	return source.Channel(s.eventChannel, handler)
}

// Start begins simple event forwarding - implements manager.Runnable interface.
func (s *SkrRuntimeEventService) Start(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Start the listener with the context (if listener exists)
	if s.listener != nil {
		err := s.listener.Start(childCtx)
		if err != nil {
			return fmt.Errorf("failed to start SKR event listener: %w", err)
		}
	}

	// Start simple forwarding (no pub/sub distribution)
	go s.forwardEvents(childCtx)

	return nil
}

// Stop shuts down - managed automatically by controller-runtime manager
// Implements manager.Runnable interface.
func (s *SkrRuntimeEventService) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}

	return nil
}

// forwardEvents - Direct 1:1 forwarding.
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
