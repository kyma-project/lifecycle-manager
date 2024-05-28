package event

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/record"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TestKymaEvent_Normal(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	kymaEvent := NewKymaEvent(fakeRecorder)

	k := &v1beta2.Kyma{}
	kymaEvent.Normal(k, "test")

	event := <-fakeRecorder.Events
	expected := "Normal test"
	assert.Contains(t, event, expected)
}

func TestKymaEvent_Normal_NoKyma(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	kymaEvent := NewKymaEvent(fakeRecorder)

	kymaEvent.Normal(nil, "test")

	assert.Equal(t, 0, len(fakeRecorder.Events))
}

func TestKymaEvent_Warning(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	kymaEvent := NewKymaEvent(fakeRecorder)

	k := &v1beta2.Kyma{}
	err := errors.New("error")
	kymaEvent.Warning(k, "test", err)

	event := <-fakeRecorder.Events
	expected := "Warning test error"
	assert.Contains(t, event, expected)
}

func TestKymaEvent_Warning_NoKyma(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	kymaEvent := NewKymaEvent(fakeRecorder)

	err := errors.New("error")
	kymaEvent.Warning(nil, "test", err)

	assert.Equal(t, 0, len(fakeRecorder.Events))
}

func TestKymaEvent_Warning_NoError(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	kymaEvent := NewKymaEvent(fakeRecorder)

	k := &v1beta2.Kyma{}
	kymaEvent.Warning(k, "test", nil)

	assert.Equal(t, 0, len(fakeRecorder.Events))
}
