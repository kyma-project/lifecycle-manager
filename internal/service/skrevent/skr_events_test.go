package skrevent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/internal/service/skrevent"
)

// Minimal tests to verify basic functionality.
func TestBasicServiceCreation(t *testing.T) {
	service := skrevent.NewSkrRuntimeEventService(nil)
	assert.NotNil(t, service, "Service should be created")
}

func TestBasicServiceStop(t *testing.T) {
	service := skrevent.NewSkrRuntimeEventService(nil)
	err := service.Stop()
	assert.NoError(t, err, "Stop should not error")
}

func TestBasicEventSource(t *testing.T) {
	service := skrevent.NewSkrRuntimeEventService(nil)
	source := service.CreateEventSource(nil)
	assert.NotNil(t, source, "EventSource should be created")
}
