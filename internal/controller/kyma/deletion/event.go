package deletion

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
)

const Success = "Success"

type EventRepo interface {
	Create(ctx context.Context, involvedObject corev1.ObjectReference, eventType, reason, message string)
}

type EventWriter struct {
	eventRepo EventRepo
}

func NewEventWriter(eventRepo EventRepo) *EventWriter {
	return &EventWriter{
		eventRepo: eventRepo,
	}
}

// Write writes a K8s event for the given Kyma resource.
// The event type is "Normal" if the result has no error, otherwise "Warning".
// The reason is set to the use case of the result.
// The message is either "Success" or the message of the error when present.
func (e *EventWriter) Write(ctx context.Context, kyma *v1beta2.Kyma, res result.Result) {
	eventType := corev1.EventTypeNormal
	message := Success

	if res.Err != nil {
		eventType = corev1.EventTypeWarning
		message = res.Err.Error()
	}

	objRef := corev1.ObjectReference{
		Kind:       kyma.Kind,
		Namespace:  kyma.Namespace,
		Name:       kyma.Name,
		UID:        kyma.UID,
		APIVersion: kyma.APIVersion,
	}

	e.eventRepo.Create(ctx,
		objRef,
		eventType,
		string(res.UseCase),
		message)
}
