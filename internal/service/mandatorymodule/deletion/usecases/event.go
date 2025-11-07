package usecases

import (
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/internal/event"
)

type EventHandler interface {
	Warning(object machineryruntime.Object, reason event.Reason, err error)
}
