package event

import (
	"k8s.io/client-go/tools/record"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type KymaEvent struct {
	record.EventRecorder
}

const (
	ModuleReconciliationError eventReason = "ModuleReconciliationError"
	MetricsError              eventReason = "MetricsError"
	UpdateSpecError           eventReason = "UpdateSpecError"
	UpdateStatusError         eventReason = "UpdateStatusError"
	PatchStatusError          eventReason = "PatchStatus"
	maxErrorLength            int         = 50
)

func NewKymaEvent(recorder record.EventRecorder) *KymaEvent {
	return &KymaEvent{recorder}
}

func (e *KymaEvent) Normal(obj *v1beta2.Kyma, reason eventReason) {
	if obj == nil {
		return
	}
	e.EventRecorder.Event(obj, typeNormal, string(reason), "")
}

func (e *KymaEvent) Warning(obj *v1beta2.Kyma, reason eventReason, err error) {
	if obj == nil || err == nil {
		return
	}
	e.EventRecorder.Event(obj, typeWarning, string(reason), truncatedErrMsg(err))
}

func truncatedErrMsg(err error) string {
	msg := err.Error()
	length := len(msg)

	if length <= maxErrorLength {
		return msg
	}

	return msg[length-maxErrorLength:]
}
