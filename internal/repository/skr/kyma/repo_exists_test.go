package kyma_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	skrkymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/kyma"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestExists_ClientCallSucceeds_ReturnsTrue(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	time := apimetav1.NewTime(time.Now())
	clientStub := &getClientStub{
		object: &v1beta1.PartialObjectMetadata{
			ObjectMeta: apimetav1.ObjectMeta{
				DeletionTimestamp: &time,
			},
		},
	}

	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrkymarepo.NewRepository(clientRetrieverStub.retrieverFunc())

	result, err := repo.Exists(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.True(t, result)
	assert.True(t, clientStub.called)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientRetrieverStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, types.NamespacedName{
		Name:      shared.DefaultRemoteKymaName,
		Namespace: shared.DefaultRemoteNamespace,
	}, clientStub.receivedKey)
}

func TestExists_ClientCallFailsWithNotFound_ReturnsFalse(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	time := apimetav1.NewTime(time.Now())
	clientStub := &getClientStub{
		object: &v1beta1.PartialObjectMetadata{
			ObjectMeta: apimetav1.ObjectMeta{
				DeletionTimestamp: &time,
			},
		},
		err: apierrors.NewNotFound(schema.GroupResource{
			Group:    v1beta2.GroupVersion.Group,
			Resource: string(shared.KymaKind),
		}, random.Name()),
	}

	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrkymarepo.NewRepository(clientRetrieverStub.retrieverFunc())

	result, err := repo.Exists(t.Context(), kcpKymaName)

	require.NoError(t, err)
	assert.False(t, result)
	assert.True(t, clientStub.called)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientRetrieverStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, types.NamespacedName{
		Name:      shared.DefaultRemoteKymaName,
		Namespace: shared.DefaultRemoteNamespace,
	}, clientStub.receivedKey)
}

func TestExists_ClientCallFails_ReturnsTrue(t *testing.T) {
	kcpKymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}

	time := apimetav1.NewTime(time.Now())
	clientStub := &getClientStub{
		object: &v1beta1.PartialObjectMetadata{
			ObjectMeta: apimetav1.ObjectMeta{
				DeletionTimestamp: &time,
			},
		},
		err: assert.AnError,
	}

	clientRetrieverStub := &skrClientRetrieverStub{
		client: clientStub,
	}

	repo := skrkymarepo.NewRepository(clientRetrieverStub.retrieverFunc())

	result, err := repo.Exists(t.Context(), kcpKymaName)

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, result)
	assert.True(t, clientStub.called)
	// kcpKymaName used to get the client
	assert.Equal(t, kcpKymaName, clientRetrieverStub.receivedKey)
	// standard Kyma name used to get the Kyma from SKR
	assert.Equal(t, types.NamespacedName{
		Name:      shared.DefaultRemoteKymaName,
		Namespace: shared.DefaultRemoteNamespace,
	}, clientStub.receivedKey)
}

type getClientStub struct {
	client.Client

	called bool
	object *v1beta1.PartialObjectMetadata
	err    error

	receivedKey client.ObjectKey
}

func (c *getClientStub) Get(_ context.Context,
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
