package kyma_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestIsDeleting_ClientCallSucceeds_ReturnsTrue(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	time := apimetav1.NewTime(time.Now())
	clientStub := &isDeletingClientStub{
		object: &v1beta1.PartialObjectMetadata{
			ObjectMeta: apimetav1.ObjectMeta{
				DeletionTimestamp: &time,
			},
		},
	}

	clientCacheStub := &skrClientCacheStub{
		client: clientStub,
	}

	repo := kyma.NewRepository(clientCacheStub)

	result, err := repo.IsDeleting(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.True(t, result)
	assert.True(t, clientStub.called)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientCacheStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, types.NamespacedName{
		Name:      shared.DefaultRemoteKymaName,
		Namespace: shared.DefaultRemoteNamespace,
	}, clientStub.receivedKey)
}

func TestIsDeleting_ClientCallSucceeds_ReturnsFalse(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	clientStub := &isDeletingClientStub{
		object: &v1beta1.PartialObjectMetadata{
			ObjectMeta: apimetav1.ObjectMeta{
				DeletionTimestamp: nil,
			},
		},
	}

	clientCacheStub := &skrClientCacheStub{
		client: clientStub,
	}

	repo := kyma.NewRepository(clientCacheStub)

	result, err := repo.IsDeleting(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.False(t, result)
	assert.True(t, clientStub.called)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientCacheStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, types.NamespacedName{
		Name:      shared.DefaultRemoteKymaName,
		Namespace: shared.DefaultRemoteNamespace,
	}, clientStub.receivedKey)
}

func TestIsDeleting_ClientReturnsAnError(t *testing.T) {
	clientStub := &isDeletingClientStub{
		err: assert.AnError,
	}
	repo := kyma.NewRepository(&skrClientCacheStub{
		client: clientStub,
	})

	result, err := repo.IsDeleting(t.Context(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	assert.False(t, result)
	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}

func TestIsDeleting_ClientNotFound_ReturnsError(t *testing.T) {
	repo := kyma.NewRepository(&skrClientCacheStub{
		client: nil, // No client available in the cache
	})

	result, err := repo.IsDeleting(t.Context(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	assert.False(t, result)
	require.Error(t, err)
	require.ErrorIs(t, err, errors.ErrSkrClientNotFound)
}

type isDeletingClientStub struct {
	client.Client

	called bool
	object *v1beta1.PartialObjectMetadata
	err    error

	receivedKey client.ObjectKey
}

func (c *isDeletingClientStub) Get(_ context.Context,
	key client.ObjectKey,
	obj client.Object,
	_ ...client.GetOption,
) error {
	c.called = true
	c.receivedKey = key
	if c.object != nil {
		c.object.DeepCopyInto(obj.(*v1beta1.PartialObjectMetadata))
	}
	return c.err
}
