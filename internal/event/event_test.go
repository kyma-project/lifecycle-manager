package event_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/record"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
)

func TestKymaEvent_Normal(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	eventRec.Normal(k, "test", "")

	events := <-fakeRecorder.Events
	expected := "Normal test"
	assert.Contains(t, events, expected)
}

func TestKymaEvent_Normal_NoKyma(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	eventRec.Normal(nil, "test", "")

	assert.Empty(t, fakeRecorder.Events)
}

func TestKymaEvent_Warning(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	err := errors.New("error")
	eventRec.Warning(k, "test", err)

	events := <-fakeRecorder.Events
	expected := "Warning test error"
	assert.Contains(t, events, expected)
}

func TestKymaEvent_Warning_NoKyma(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	err := errors.New("error")
	eventRec.Warning(nil, "test", err)

	assert.Empty(t, fakeRecorder.Events)
}

func TestKymaEvent_Warning_NoError(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	eventRec.Warning(k, "test", nil)

	assert.Empty(t, fakeRecorder.Events)
}
