package status_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
	skrkymastatusrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma/status"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestGet_ClientCallSucceeds_ReturnsKymaStatus(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	clientStub := &getClientStub{
		object: &v1beta2.Kyma{
			Status: v1beta2.KymaStatus{
				State: shared.StateReady,
			},
		},
	}

	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrkymastatusrepo.NewRepository(clientRetrieverStub.retrieverFunc())

	result, err := repo.Get(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shared.StateReady, result.State)
	assert.True(t, clientStub.called)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientRetrieverStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, types.NamespacedName{
		Name:      shared.DefaultRemoteKymaName,
		Namespace: shared.DefaultRemoteNamespace,
	}, clientStub.receivedKey)
}

func TestGet_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		err: assert.AnError,
	}
	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}
	repo := skrkymastatusrepo.NewRepository(clientRetrieverStub.retrieverFunc())

	result, err := repo.Get(t.Context(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	assert.Nil(t, result)
	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}

func TestGet_ClientNotFound_ReturnsError(t *testing.T) {
	clientRetrieverStub := &skrClientRetrieverStub{
		client: nil, // No client available in the cache
	}
	repo := skrkymastatusrepo.NewRepository(clientRetrieverStub.retrieverFunc())

	result, err := repo.Get(t.Context(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	assert.Nil(t, result)
	require.Error(t, err)
	require.ErrorIs(t, err, errorsinternal.ErrSkrClientNotFound)
}

type getClientStub struct {
	client.Client

	called bool
	object *v1beta2.Kyma
	err    error

	receivedKey client.ObjectKey
}

func (c *getClientStub) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.called = true
	c.receivedKey = key
	if c.object != nil {
		c.object.DeepCopyInto(obj.(*v1beta2.Kyma))
	}
	return c.err
}
