package secret_test

import (
	"context"
	"errors"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate/secret"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
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

func (c *kcpClientStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object,
	opts ...client.GetOption,
) error {
	c.getCalled = true
	if c.getSecret != nil {
		//nolint:forcetypeassert // test code
		c.getSecret.DeepCopyInto(obj.(*apicorev1.Secret))
	}
	return c.getErr
}

func (c *kcpClientStub) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalled = true
	//nolint:forcetypeassert // test code
	c.deleteArg = obj.(*apicorev1.Secret)
	return c.deleteErr
}

func TestNewGatewaySecretData(t *testing.T) {
	caData := []byte("ca-cert-data")
	tests := []struct {
		name    string
		secret  *apicorev1.Secret
		want    *secret.GatewaySecretData
		wantErr bool
	}{
		{
			name:    "nil secret",
			secret:  nil,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nil data map",
			secret:  &apicorev1.Secret{Data: nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing ca.crt key",
			secret:  &apicorev1.Secret{Data: map[string][]byte{}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "ca.crt present",
			secret:  &apicorev1.Secret{Data: map[string][]byte{secret.CaCertKey: caData}},
			want:    &secret.GatewaySecretData{CaCert: caData},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := secret.NewGatewaySecretData(testCase.secret)
			if testCase.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.want, got)
			}
		})
	}
}

func TestNewCertificateSecretData(t *testing.T) {
	tlsCert := []byte("tls-cert")
	tlsKey := []byte("tls-key")
	tests := []struct {
		name    string
		secret  *apicorev1.Secret
		want    *secret.CertificateSecretData
		wantErr bool
	}{
		{
			name:    "nil secret",
			secret:  nil,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "nil data map",
			secret:  &apicorev1.Secret{Data: nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing tls.crt key",
			secret:  &apicorev1.Secret{Data: map[string][]byte{secret.TlsPrivateKeyKey: tlsKey}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing tls.key key",
			secret:  &apicorev1.Secret{Data: map[string][]byte{secret.TlsCertKey: tlsCert}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "both keys present",
			secret: &apicorev1.Secret{
				Data: map[string][]byte{
					secret.TlsCertKey:       tlsCert,
					secret.TlsPrivateKeyKey: tlsKey,
				},
			},
			want:    &secret.CertificateSecretData{TlsCert: tlsCert, TlsKey: tlsKey},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := secret.NewCertificateSecretData(testCase.secret)
			if testCase.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.want, got)
			}
		})
	}
}
