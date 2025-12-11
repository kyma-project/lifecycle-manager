package event_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/record"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
)

func TestNormalEvent_CalledWithObject(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	eventRec.Normal(k, "test", "")

	events := <-fakeRecorder.Events
	expected := "Normal test"
	assert.Contains(t, events, expected)
}

func TestNormalEvent_CalledWithNilObject(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	eventRec.Normal(nil, "test", "")

	assert.Empty(t, fakeRecorder.Events)
}

func TestWarningEvent_CalledWithNilObject(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	err := errors.New("error")
	eventRec.Warning(nil, "test", err)

	assert.Empty(t, fakeRecorder.Events)
}

func TestWarningEvent_CalledWithNilError(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	eventRec.Warning(k, "test", nil)

	assert.Empty(t, fakeRecorder.Events)
}

func TestWarningEvent_CalledWithErrorMsg50(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	err := errors.New("12345678901234567890123456789012345678901234567890")
	eventRec.Warning(k, "test", err)

	events := <-fakeRecorder.Events
	expected := "Warning test 12345678901234567890123456789012345678901234567890"
	assert.Contains(t, events, expected)
}

func TestWarningEvent_CalledWithErrorMsgLongerThanMax(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	err := errors.New("this is a very long error message that should be truncated at the end because it exceeds the maximum length allowed for event messages in Kubernetes")
	eventRec.Warning(k, "test", err)

	events := <-fakeRecorder.Events
	expected := "Warning test  truncated at the end because it exceeds the maximum length allowed for event messages in Kubernetes"
	assert.Contains(t, events, expected)
}

func TestWarningEvent_CalledWithEmptyErrorMsg(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	eventRec := event.NewRecorderWrapper(fakeRecorder)

	k := &v1beta2.Kyma{}
	err := errors.New("")
	eventRec.Warning(k, "test", err)

	events := <-fakeRecorder.Events
	expected := "Warning test"
	assert.Contains(t, events, expected)
}
