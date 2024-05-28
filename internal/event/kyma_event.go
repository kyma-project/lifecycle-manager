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
)

func NewKymaEvent(recorder record.EventRecorder) *KymaEvent {
	return &KymaEvent{recorder}
}

func (e *KymaEvent) Normal(kyma *v1beta2.Kyma, reason eventReason) {
	if kyma == nil {
		return
	}
	e.EventRecorder.Event(kyma, typeNormal, string(reason), "")
}

func (e *KymaEvent) Warning(kyma *v1beta2.Kyma, reason eventReason, err error) {
	if kyma == nil || err == nil {
		return
	}
	e.EventRecorder.Event(kyma, typeWarning, string(reason), truncatedErrMsg(err))
}
