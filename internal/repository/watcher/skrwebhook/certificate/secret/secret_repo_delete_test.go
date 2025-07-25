package secret_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/secret"
)

func Test_CertificateSecretClient_Delete_Success(t *testing.T) {
	clientStub := &deleteClientStub{}
	secretClient := secret.NewCertificateSecretRepository(clientStub, namespace)

	err := secretClient.Delete(t.Context(), secretName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.calledWith)
	assert.Equal(t, secretName, clientStub.calledWith.Name)
	assert.Equal(t, namespace, clientStub.calledWith.Namespace)
}

func Test_CertificateSecretClient_Delete_Error(t *testing.T) {
	clientStub := &deleteClientStub{
		err: errors.New("failed to delete secret"),
	}
	secretClient := secret.NewCertificateSecretRepository(clientStub, namespace)

	err := secretClient.Delete(t.Context(), secretName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete secret")
	assert.True(t, clientStub.called)
}

func Test_CertificateSecretClient_Delete_IgnoreNotFoundError(t *testing.T) {
	clientStub := &deleteClientStub{
		err: apierrors.NewNotFound(apicorev1.Resource("secrets"), secretName),
	}
	secretClient := secret.NewCertificateSecretRepository(clientStub, namespace)

	err := secretClient.Delete(t.Context(), secretName)

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
