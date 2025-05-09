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

	"github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var (
	secretName      = random.Name()
	secretNamespace = random.Name()
)

func Test_CertificateSecretClient_Get_Success(t *testing.T) {
	kcpStub := &kcpStub{
		getSecret: &apicorev1.Secret{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
		},
	}
	secretClient := secret.NewCertificateSecretClient(kcpStub)

	result, err := secretClient.Get(t.Context(), secretName, secretNamespace)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, secretName, result.Name)
	assert.Equal(t, secretNamespace, result.Namespace)
	assert.True(t, kcpStub.getCalled)
}

func Test_CertificateSecretClient_Get_Error(t *testing.T) {
	kcpStub := &kcpStub{
		getErr: errors.New("failed to get secret"),
	}
	secretClient := secret.NewCertificateSecretClient(kcpStub)

	result, err := secretClient.Get(t.Context(), secretName, secretNamespace)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get secret")
	assert.True(t, kcpStub.getCalled)
}

func Test_CertificateSecretClient_Delete_Success(t *testing.T) {
	kcpStub := &kcpStub{}
	secretClient := secret.NewCertificateSecretClient(kcpStub)

	err := secretClient.Delete(t.Context(), secretName, secretNamespace)

	require.NoError(t, err)
	assert.True(t, kcpStub.deleteCalled)
	assert.NotNil(t, kcpStub.deleteArg)
	assert.Equal(t, secretName, kcpStub.deleteArg.Name)
	assert.Equal(t, secretNamespace, kcpStub.deleteArg.Namespace)
}

func Test_CertificateSecretClient_Delete_Error(t *testing.T) {
	kcpStub := &kcpStub{
		deleteErr: errors.New("failed to delete secret"),
	}
	secretClient := secret.NewCertificateSecretClient(kcpStub)

	err := secretClient.Delete(t.Context(), secretName, secretNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete secret")
	assert.True(t, kcpStub.deleteCalled)
}

func Test_CertificateSecretClient_Delete_IgnoreNotFoundError(t *testing.T) {
	kcpStub := &kcpStub{
		deleteErr: apierrors.NewNotFound(apicorev1.Resource("secrets"), secretName),
	}
	secretClient := secret.NewCertificateSecretClient(kcpStub)

	err := secretClient.Delete(t.Context(), secretName, secretNamespace)

	require.NoError(t, err)
	assert.True(t, kcpStub.deleteCalled)
	assert.NotNil(t, kcpStub.deleteArg)
	assert.Equal(t, secretName, kcpStub.deleteArg.Name)
	assert.Equal(t, secretNamespace, kcpStub.deleteArg.Namespace)
}

// Test stubs

type kcpStub struct {
	client.Client
	getSecret    *apicorev1.Secret
	getCalled    bool
	getErr       error
	deleteCalled bool
	deleteErr    error
	deleteArg    *apicorev1.Secret
}

func (c *kcpStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCalled = true
	if c.getSecret != nil {
		//nolint:forcetypeassert // test code
		c.getSecret.DeepCopyInto(obj.(*apicorev1.Secret))
	}
	return c.getErr
}

func (c *kcpStub) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalled = true
	//nolint:forcetypeassert // test code
	c.deleteArg = obj.(*apicorev1.Secret)
	return c.deleteErr
}
