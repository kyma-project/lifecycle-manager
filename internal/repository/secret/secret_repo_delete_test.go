package secret_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/secret"
)

func TestDelete_ClientCallSucceeds_Returns(t *testing.T) {
	clientStub := &deleteClientStub{}
	secretRepository := secret.NewRepository(clientStub, namespace)

	err := secretRepository.Delete(t.Context(), secretName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.calledWith)
	assert.Equal(t, secretName, clientStub.calledWith.Name)
	assert.Equal(t, namespace, clientStub.calledWith.Namespace)
}

func TestDelete_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &deleteClientStub{
		err: assert.AnError,
	}
	secretRepository := secret.NewRepository(clientStub, namespace)

	err := secretRepository.Delete(t.Context(), secretName)

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}

func TestDelete_ClientReturnsNotFoundError_Returns(t *testing.T) {
	clientStub := &deleteClientStub{
		err: apierrors.NewNotFound(apicorev1.Resource("secrets"), secretName),
	}
	secretRepository := secret.NewRepository(clientStub, namespace)

	err := secretRepository.Delete(t.Context(), secretName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.calledWith)
	assert.Equal(t, secretName, clientStub.calledWith.Name)
	assert.Equal(t, namespace, clientStub.calledWith.Namespace)
}

type deleteClientStub struct {
	client.Client
	called     bool
	calledWith *apicorev1.Secret
	err        error
}

func (c *deleteClientStub) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	c.called = true
	c.calledWith = obj.(*apicorev1.Secret)
	return c.err
}
