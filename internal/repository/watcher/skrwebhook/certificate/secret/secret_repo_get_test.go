package secret_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/secret"
)

func TestGet_WhenCalledAndClientCallSucceeds_ReturnsSecret(t *testing.T) {
	clientStub := &getClientStub{
		object: &apicorev1.Secret{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
		},
	}
	secretClient := secret.NewCertificateSecretRepository(clientStub, namespace)

	result, err := secretClient.Get(t.Context(), secretName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, secretName, result.Name)
	assert.Equal(t, namespace, result.Namespace)
	assert.True(t, clientStub.called)
}

func Test_CertificateSecretClient_Get_Error(t *testing.T) {
	clientStub := &getClientStub{
		err: errors.New("failed to get secret"),
	}
	secretClient := secret.NewCertificateSecretRepository(clientStub, namespace)

	result, err := secretClient.Get(t.Context(), secretName)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get secret")
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
