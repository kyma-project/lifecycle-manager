package crd_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
	skrcrdrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/crd"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestExists_ClientCallSucceeds_ReturnsTrue(t *testing.T) {
	kymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}
	crdName := random.Name()

	clientStub := &getClientStub{
		object: &v1beta1.PartialObjectMetadata{},
	}

	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrcrdrepo.NewRepository(clientRetrieverStub.retrieverFunc(), crdName)

	result, err := repo.Exists(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t, kymaName, clientRetrieverStub.receivedKey)
	assert.Equal(t, types.NamespacedName{Name: crdName}, clientStub.receivedKey)
}

func TestExists_ClientCallFailsWithNotFound_ReturnsFalse(t *testing.T) {
	kymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}
	crdName := random.Name()

	clientStub := &getClientStub{
		err: apierrors.NewNotFound(schema.GroupResource{
			Group:    apiextensionsv1.SchemeGroupVersion.Group,
			Resource: reflect.TypeFor[apiextensionsv1.CustomResourceDefinition]().Name(),
		}, crdName),
	}

	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrcrdrepo.NewRepository(clientRetrieverStub.retrieverFunc(), crdName)

	result, err := repo.Exists(t.Context(), kymaName)

	require.NoError(t, err)
	assert.False(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t, kymaName, clientRetrieverStub.receivedKey)
	assert.Equal(t, types.NamespacedName{Name: crdName}, clientStub.receivedKey)
}

func TestExists_ClientCallFailsWithOtherError_ReturnsError(t *testing.T) {
	kymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}
	crdName := random.Name()

	clientStub := &getClientStub{
		err: assert.AnError,
	}

	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrcrdrepo.NewRepository(clientRetrieverStub.retrieverFunc(), crdName)

	result, err := repo.Exists(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t, kymaName, clientRetrieverStub.receivedKey)
	assert.Equal(t, types.NamespacedName{Name: crdName}, clientStub.receivedKey)
}

func TestExists_ClientNotFound_ReturnsError(t *testing.T) {
	clientRetrieverStub := &skrClientRetrieverStub{
		client: nil, // No client available in the cache
	}
	repo := skrcrdrepo.NewRepository(clientRetrieverStub.retrieverFunc(), random.Name())

	result, err := repo.Exists(context.Background(),
		types.NamespacedName{
			Name:      random.Name(),
			Namespace: random.Name(),
		})

	require.Error(t, err)
	require.ErrorIs(t, err, errorsinternal.ErrSkrClientNotFound)
	require.False(t, result)
}

type getClientStub struct {
	client.Client

	called bool
	object *v1beta1.PartialObjectMetadata
	err    error

	receivedKey client.ObjectKey
}

func (c *getClientStub) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.called = true
	c.receivedKey = key
	if c.object != nil {
		c.object.DeepCopyInto(obj.(*v1beta1.PartialObjectMetadata))
	}
	return c.err
}
