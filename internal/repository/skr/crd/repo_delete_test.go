package crd_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/errors"
	skrcrdrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/crd"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestDelete_ClientCallSucceeds(t *testing.T) {
	kymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}
	crdName := random.Name()

	clientStub := &deleteClientStub{}
	clientCacheStub := &skrClientCacheStub{
		client: clientStub,
	}

	repo := skrcrdrepo.NewRepository(clientCacheStub, crdName)

	err := repo.Delete(context.Background(), kymaName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	// kymaName used to get the client
	assert.Equal(t, kymaName, clientCacheStub.receivedKey)
	// CRD name used to delete the CRD
	assert.Equal(t, crdName, clientStub.deletedObject.GetName())
}

func TestDelete_ClientReturnsAnError(t *testing.T) {
	clientStub := &deleteClientStub{
		err: assert.AnError,
	}
	repo := skrcrdrepo.NewRepository(&skrClientCacheStub{
		client: clientStub,
	}, random.Name())

	err := repo.Delete(context.Background(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}

func TestDelete_ClientIgnoresNotFoundError(t *testing.T) {
	clientStub := &deleteClientStub{
		err: apierrors.NewNotFound(schema.GroupResource{
			Group:    apiextensionsv1.SchemeGroupVersion.Group,
			Resource: reflect.TypeOf(apiextensionsv1.CustomResourceDefinition{}).Name(),
		}, random.Name()),
	}
	repo := skrcrdrepo.NewRepository(&skrClientCacheStub{
		client: clientStub,
	}, random.Name())

	err := repo.Delete(context.Background(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.NoError(t, err)
	assert.True(t, clientStub.called)
}

func TestDelete_ClientNotFound_ReturnsError(t *testing.T) {
	repo := skrcrdrepo.NewRepository(&skrClientCacheStub{
		client: nil, // No client available in the cache
	}, random.Name())

	err := repo.Delete(context.Background(),
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
