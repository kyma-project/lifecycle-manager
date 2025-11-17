package event_test

import (
	"context"
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/repository/event"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clientStub struct {
	client.Client

	createErr error

	createCalled  bool
	receivedEvent *corev1.Event
}

func (c *clientStub) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	c.createCalled = true

	if event, ok := obj.(*corev1.Event); ok {
		c.receivedEvent = &corev1.Event{}
		event.DeepCopyInto(c.receivedEvent)
	}

	return c.createErr
}

func TestRepository_Create_Success(t *testing.T) {
	clntStub := &clientStub{}
	namespace := random.Name()
	eventName := random.Name()
	reason := random.Name()
	message := random.Name()

	repo := event.NewRepository(clntStub, namespace, eventName)

	involvedObject := corev1.ObjectReference{
		Kind:       random.Name(),
		Namespace:  namespace,
		Name:       random.Name(),
		UID:        uuid.NewUUID(),
		APIVersion: "v1",
	}

	repo.Create(t.Context(),
		involvedObject,
		corev1.EventTypeNormal,
		reason,
		message,
	)

	assert.True(t, clntStub.createCalled)
	assert.NotNil(t, clntStub.receivedEvent)
	assert.Equal(t, corev1.EventTypeNormal, clntStub.receivedEvent.Type)
	assert.Equal(t, reason, clntStub.receivedEvent.Reason)
	assert.Equal(t, message, clntStub.receivedEvent.Message)
	assert.Equal(t, involvedObject, clntStub.receivedEvent.InvolvedObject)
	assert.Equal(t, namespace, clntStub.receivedEvent.Namespace)
	assert.Contains(t, clntStub.receivedEvent.Name, eventName)
}

func TestRepository_Create_Swallows_Failure(t *testing.T) {
	err := assert.AnError
	clntStub := &clientStub{
		createErr: err,
	}

	repo := event.NewRepository(clntStub, random.Name(), random.Name())

	repo.Create(t.Context(),
		corev1.ObjectReference{},
		corev1.EventTypeNormal,
		random.Name(),
		random.Name(),
	)

	assert.True(t, clntStub.createCalled)
}
