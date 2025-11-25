package kyma_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestDelete_ClientCallSucceeds(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	clientStub := &deleteClientStub{}
	clientCacheStub := &skrClientCacheStub{
		client: clientStub,
	}

	repo := kyma.NewRepository(clientCacheStub)

	err := repo.Delete(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientCacheStub.receivedKey)
	// standard Kyma name used to delete the Kyma from SKR
	assert.Equal(t, &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.DefaultRemoteKymaName,
			Namespace: shared.DefaultRemoteNamespace,
		},
	}, clientStub.deletedObject)
}

func TestDelete_ClientReturnsAnError(t *testing.T) {
	clientStub := &deleteClientStub{
		err: assert.AnError,
	}
	repo := kyma.NewRepository(&skrClientCacheStub{
		client: clientStub,
	})

	err := repo.Delete(t.Context(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}

func TestDelete_ClientNotFound_ReturnsError(t *testing.T) {
	repo := kyma.NewRepository(&skrClientCacheStub{
		client: nil, // No client available in the cache
	})

	err := repo.Delete(t.Context(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.Error(t, err)
	require.ErrorIs(t, err, errors.ErrSkrClientNotFound)
}

type deleteClientStub struct {
	client.Client

	called        bool
	deletedObject client.Object
	err           error
}

func (c *deleteClientStub) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	c.called = true
	c.deletedObject = obj
	return c.err
}
