package usecases

import (
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"k8s.io/apimachinery/pkg/runtime"
)

type EventHandler interface {
	Warning(object runtime.Object, reason event.Reason, err error)
}
