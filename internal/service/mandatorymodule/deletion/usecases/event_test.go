package usecases_test

import (
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/internal/event"
)

type mockEventHandler struct {
	Called bool
	Reason event.Reason
}

func (m *mockEventHandler) Warning(_ machineryruntime.Object, reason event.Reason, _ error) {
	m.Called = true
	m.Reason = reason
}
