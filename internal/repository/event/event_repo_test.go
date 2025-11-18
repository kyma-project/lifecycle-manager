package event_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/event"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type clientStub struct {
	client.Client

	createErr error

	createCalled  bool
	receivedEvent *apicorev1.Event
}

func (c *clientStub) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	c.createCalled = true

	if event, ok := obj.(*apicorev1.Event); ok {
		c.receivedEvent = &apicorev1.Event{}
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

	involvedObject := apicorev1.ObjectReference{
		Kind:       random.Name(),
		Namespace:  namespace,
		Name:       random.Name(),
		UID:        uuid.NewUUID(),
		APIVersion: "v1",
	}

	repo.Create(t.Context(),
		involvedObject,
		apicorev1.EventTypeNormal,
		reason,
		message,
	)

	assert.True(t, clntStub.createCalled)
	assert.NotNil(t, clntStub.receivedEvent)
	assert.Equal(t, apicorev1.EventTypeNormal, clntStub.receivedEvent.Type)
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
		apicorev1.ObjectReference{},
		apicorev1.EventTypeNormal,
		random.Name(),
		random.Name(),
	)

	assert.True(t, clntStub.createCalled)
}
