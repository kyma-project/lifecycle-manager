package usecases

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/internal/event"
)

type EventHandler interface {
	Warning(object runtime.Object, reason event.Reason, err error)
}
