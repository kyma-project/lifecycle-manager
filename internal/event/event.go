package event

import (
	apicorev1 "k8s.io/api/core/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
)

type Event interface {
	Normal(object machineryruntime.Object, reason Reason, msg string)
	Warning(object machineryruntime.Object, reason Reason, err error)
}

type Reason string

const (
	maxErrorLength int = 100
)

type RecorderWrapper struct {
	recorder events.EventRecorder
}

func NewRecorderWrapper(recorder events.EventRecorder) *RecorderWrapper {
	return &RecorderWrapper{recorder}
}

func (e *RecorderWrapper) Normal(obj machineryruntime.Object, reason Reason, msg string) {
	if obj == nil {
		return
	}
	var related machineryruntime.Object = nil // related object is optional. We may consider using it in the future.
	action := string(reason)
	e.recorder.Eventf(obj, related, apicorev1.EventTypeNormal, string(reason), action, msg)
}

func (e *RecorderWrapper) Warning(obj machineryruntime.Object, reason Reason, err error) {
	if obj == nil || err == nil {
		return
	}
	var related machineryruntime.Object = nil // related object is optional. We may consider using it in the future.
	action := string(reason) 
	e.recorder.Eventf(obj, related, apicorev1.EventTypeWarning, string(reason), action, truncatedErrMsg(err))
}

func truncatedErrMsg(err error) string {
	msg := err.Error()
	length := len(msg)

	if length <= maxErrorLength {
		return msg
	}

	return msg[length-maxErrorLength:]
}
