package deletion_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type eventRepoStub struct {
	createCalled bool

	involvedObject corev1.ObjectReference
	eventType      string
	reason         string
	message        string
}

func (e *eventRepoStub) Create(_ context.Context, involvedObject corev1.ObjectReference, eventType, reason, message string) {
	e.createCalled = true
	e.involvedObject = involvedObject
	e.eventType = eventType
	e.reason = reason
	e.message = message
}

func TestEventWriter_Write_NormalEvent_Success(t *testing.T) {
	res := result.Result{
		UseCase: result.UseCase(random.Name()),
		Err:     nil,
	}

	repoStub := &eventRepoStub{}

	kyma := builder.NewKymaBuilder().
		WithName(random.Name()).
		WithNamespace(random.Name()).
		WithUid(uuid.NewUUID()).
		Build()

	event := deletion.NewEventWriter(repoStub)

	event.Write(t.Context(), kyma, res)

	assert.True(t, repoStub.createCalled)
	assert.Equal(t, corev1.EventTypeNormal, repoStub.eventType)
	assert.Equal(t, string(res.UseCase), repoStub.reason)
	assert.Equal(t, "Success", repoStub.message)
	assert.Equal(t, kyma.Name, repoStub.involvedObject.Name)
	assert.Equal(t, kyma.Namespace, repoStub.involvedObject.Namespace)
	assert.Equal(t, kyma.UID, repoStub.involvedObject.UID)
	assert.Equal(t, kyma.Kind, repoStub.involvedObject.Kind)
	assert.Equal(t, kyma.APIVersion, repoStub.involvedObject.APIVersion)
}

func TestEventWriter_Write_WarningEvent_Success(t *testing.T) {
	err := assert.AnError
	res := result.Result{
		UseCase: result.UseCase(random.Name()),
		Err:     err,
	}

	repoStub := &eventRepoStub{}

	kyma := builder.NewKymaBuilder().
		WithName(random.Name()).
		WithNamespace(random.Name()).
		WithUid(uuid.NewUUID()).
		Build()

	event := deletion.NewEventWriter(repoStub)

	event.Write(t.Context(), kyma, res)

	assert.True(t, repoStub.createCalled)
	assert.Equal(t, corev1.EventTypeWarning, repoStub.eventType)
	assert.Equal(t, string(res.UseCase), repoStub.reason)
	assert.Equal(t, err.Error(), repoStub.message)
	assert.Equal(t, kyma.Name, repoStub.involvedObject.Name)
	assert.Equal(t, kyma.Namespace, repoStub.involvedObject.Namespace)
	assert.Equal(t, kyma.UID, repoStub.involvedObject.UID)
	assert.Equal(t, kyma.Kind, repoStub.involvedObject.Kind)
	assert.Equal(t, kyma.APIVersion, repoStub.involvedObject.APIVersion)
}
