package event

import (
	"context"
	"errors"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/internal/errors/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/result"
)

const success = "Success"

type Event interface {
	Normal(object machineryruntime.Object, reason event.Reason, msg string)
	Warning(object machineryruntime.Object, reason event.Reason, err error)
}

type EventRecorder struct {
	event Event
}

func NewEventRecorder(event Event) *EventRecorder {
	return &EventRecorder{
		event: event,
	}
}

// Record recordes a K8s event for the given object based on the provided result.
// The event type is "Normal" if the result has no error, otherwise "Warning".
// The reason is set to the use case of the result.
// The message is either "Success" or the message of the error when present.
func (e *EventRecorder) Record(ctx context.Context, object machineryruntime.Object, res result.Result) {
	if errors.Is(res.Err, deletion.ErrNoUseCaseApplicable) {
		e.event.Warning(object, event.Reason("KymaDeletion"), res.Err)
		return
	}

	if res.Err != nil {
		e.event.Warning(object, event.Reason(res.UseCase), res.Err)
		return
	}

	e.event.Normal(object, event.Reason(res.UseCase), success)
}
