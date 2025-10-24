package usecases_test

import (
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"k8s.io/apimachinery/pkg/runtime"
)

type MockEventHandler struct {
	Called bool
	Reason event.Reason
}

func (m *MockEventHandler) Warning(_ runtime.Object, reason event.Reason, _ error) {
	m.Called = true
	m.Reason = reason
}
