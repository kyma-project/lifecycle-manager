package event

import (
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type Event interface {
	Normal(object machineryruntime.Object, reason Reason, msg string)
	Warning(object machineryruntime.Object, reason Reason, err error)
}

type Reason string

const (
	typeNormal         = "Normal"
	typeWarning        = "Warning"
	maxErrorLength int = 50
)

type RecorderWrapper struct {
	recorder record.EventRecorder
}

func NewRecorderWrapper(recorder record.EventRecorder) *RecorderWrapper {
	return &RecorderWrapper{recorder}
}

func (e *RecorderWrapper) Normal(obj machineryruntime.Object, reason Reason, msg string) {
	if obj == nil {
		return
	}
	e.recorder.Event(obj, typeNormal, string(reason), msg)
}

func (e *RecorderWrapper) Warning(obj machineryruntime.Object, reason Reason, err error) {
	if obj == nil || err == nil {
		return
	}
	e.recorder.Event(obj, typeWarning, string(reason), truncatedErrMsg(err))
}

func truncatedErrMsg(err error) string {
	msg := err.Error()
	length := len(msg)

	if length <= maxErrorLength {
		return msg
	}

	return msg[length-maxErrorLength:]
}
