package common

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// EventService defines the interface for event services consumed by controllers.
// Moved to common package to avoid circular dependencies between service and controller packages.
type EventService interface {
	// Start begins the event service lifecycle
	Start(ctx context.Context) error

	// Stop shuts down the event service
	Stop() error

	// CreateEventSource creates a controller-runtime compatible event source
	CreateEventSource(eventHandler handler.EventHandler) source.Source
}
