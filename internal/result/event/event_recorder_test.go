package event_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	resultevent "github.com/kyma-project/lifecycle-manager/internal/result/event"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type eventStub struct {
	normalCalled  bool
	warningCalled bool

	involvedObject unstructured.Unstructured
	eventType      string
	reason         string
	message        string
}

func (e *eventStub) Normal(object machineryruntime.Object, reason event.Reason, msg string) {
	e.normalCalled = true
	e.eventType = apicorev1.EventTypeNormal
	e.reason = string(reason)
	e.message = msg

	unstructuredObj, err := machineryruntime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		panic(err)
	}
	e.involvedObject = unstructured.Unstructured{Object: unstructuredObj}
}

func (e *eventStub) Warning(object machineryruntime.Object, reason event.Reason, err error) {
	e.warningCalled = true
	e.eventType = apicorev1.EventTypeWarning
	e.reason = string(reason)
	e.message = err.Error()

	unstructuredObj, err := machineryruntime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		panic(err)
	}
	e.involvedObject = unstructured.Unstructured{Object: unstructuredObj}
}

func TestEventRecorder_Record_NormalEvent_Success(t *testing.T) {
	res := result.Result{
		UseCase: result.UseCase(random.Name()),
		Err:     nil,
	}

	eventStub := &eventStub{}

	kyma := builder.NewKymaBuilder().
		WithName(random.Name()).
		WithNamespace(random.Name()).
		WithUid(uuid.NewUUID()).
		Build()

	event := resultevent.NewEventRecorder(eventStub)

	event.Record(t.Context(), kyma, res)

	assert.True(t, eventStub.normalCalled)
	assert.Equal(t, apicorev1.EventTypeNormal, eventStub.eventType)
	assert.Equal(t, string(res.UseCase), eventStub.reason)
	assert.Equal(t, "Success", eventStub.message)
	assert.Equal(t, kyma.APIVersion, eventStub.involvedObject.GetAPIVersion())
	assert.Equal(t, kyma.Kind, eventStub.involvedObject.GetKind())
	assert.Equal(t, kyma.Name, eventStub.involvedObject.GetName())
	assert.Equal(t, kyma.Namespace, eventStub.involvedObject.GetNamespace())
	assert.Equal(t, kyma.GetUID(), eventStub.involvedObject.GetUID())
}

func TestEventRecorder_Record_WarningEvent_Success(t *testing.T) {
	res := result.Result{
		UseCase: result.UseCase(random.Name()),
		Err:     assert.AnError,
	}

	eventStub := &eventStub{}

	kyma := builder.NewKymaBuilder().
		WithName(random.Name()).
		WithNamespace(random.Name()).
		WithUid(uuid.NewUUID()).
		Build()

	event := resultevent.NewEventRecorder(eventStub)

	event.Record(t.Context(), kyma, res)

	assert.True(t, eventStub.warningCalled)
	assert.Equal(t, apicorev1.EventTypeWarning, eventStub.eventType)
	assert.Equal(t, string(res.UseCase), eventStub.reason)
	assert.Equal(t, res.Err.Error(), eventStub.message)
	assert.Equal(t, kyma.APIVersion, eventStub.involvedObject.GetAPIVersion())
	assert.Equal(t, kyma.Kind, eventStub.involvedObject.GetKind())
	assert.Equal(t, kyma.Name, eventStub.involvedObject.GetName())
	assert.Equal(t, kyma.Namespace, eventStub.involvedObject.GetNamespace())
	assert.Equal(t, kyma.GetUID(), eventStub.involvedObject.GetUID())
}
