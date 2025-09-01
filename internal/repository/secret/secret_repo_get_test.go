package secret_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretrepository "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
)

func TestGet_ClientCallSucceeds_ReturnsSecret(t *testing.T) {
	clientStub := &getClientStub{
		object: &apicorev1.Secret{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
		},
	}
	secretRepository := secretrepository.NewRepository(clientStub, namespace)

	result, err := secretRepository.Get(t.Context(), secretName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, secretName, result.Name)
	assert.Equal(t, namespace, result.Namespace)
	assert.True(t, clientStub.called)
}

func TestGet_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		err: assert.AnError,
	}
	secretRepository := secretrepository.NewRepository(clientStub, namespace)

	result, err := secretRepository.Get(t.Context(), secretName)

	assert.Nil(t, result)
	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}

type getClientStub struct {
	client.Client
	called bool
	object *apicorev1.Secret
	err    error
}

func (c *getClientStub) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.called = true
	if c.object != nil {
		c.object.DeepCopyInto(obj.(*apicorev1.Secret))
	}
	return c.err
}
