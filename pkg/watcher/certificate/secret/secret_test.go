package secret_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/secret"
)

var (
	secretName      = random.Name()
	secretNamespace = random.Name()
)

func Test_CertificateSecretClient_Get_Success(t *testing.T) {
	clientStub := &kcpClientStub{
		getSecret: &apicorev1.Secret{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		},
	}
	secretClient := secret.NewCertificateSecretClient(clientStub)

	result, err := secretClient.Get(t.Context(), secretName, secretNamespace)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, secretName, result.Name)
	assert.Equal(t, secretNamespace, result.Namespace)
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateSecretClient_Get_Error(t *testing.T) {
	clientStub := &kcpClientStub{
		getErr: errors.New("failed to get secret"),
	}
	secretClient := secret.NewCertificateSecretClient(clientStub)

	result, err := secretClient.Get(t.Context(), secretName, secretNamespace)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get secret")
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateSecretClient_Delete_Success(t *testing.T) {
	clientStub := &kcpClientStub{}
	secretClient := secret.NewCertificateSecretClient(clientStub)

	err := secretClient.Delete(t.Context(), secretName, secretNamespace)

	require.NoError(t, err)
	assert.True(t, clientStub.deleteCalled)
	assert.NotNil(t, clientStub.deleteArg)
	assert.Equal(t, secretName, clientStub.deleteArg.Name)
	assert.Equal(t, secretNamespace, clientStub.deleteArg.Namespace)
}

func Test_CertificateSecretClient_Delete_Error(t *testing.T) {
	clientStub := &kcpClientStub{
		deleteErr: errors.New("failed to delete secret"),
	}
	secretClient := secret.NewCertificateSecretClient(clientStub)

	err := secretClient.Delete(t.Context(), secretName, secretNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete secret")
	assert.True(t, clientStub.deleteCalled)
}

func Test_CertificateSecretClient_Delete_IgnoreNotFoundError(t *testing.T) {
	clientStub := &kcpClientStub{
		deleteErr: apierrors.NewNotFound(apicorev1.Resource("secrets"), secretName),
	}
	secretClient := secret.NewCertificateSecretClient(clientStub)

	err := secretClient.Delete(t.Context(), secretName, secretNamespace)

	require.NoError(t, err)
	assert.True(t, clientStub.deleteCalled)
	assert.NotNil(t, clientStub.deleteArg)
	assert.Equal(t, secretName, clientStub.deleteArg.Name)
	assert.Equal(t, secretNamespace, clientStub.deleteArg.Namespace)
}

// Test stubs

type kcpClientStub struct {
	getSecret    *apicorev1.Secret
	getCalled    bool
	getErr       error
	deleteCalled bool
	deleteErr    error
	deleteArg    *apicorev1.Secret
}

func (c *kcpClientStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCalled = true
	if c.getSecret != nil {
		c.getSecret.DeepCopyInto(obj.(*apicorev1.Secret))
	}
	return c.getErr
}

func (c *kcpClientStub) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalled = true
	c.deleteArg = obj.(*apicorev1.Secret)
	return c.deleteErr
}
