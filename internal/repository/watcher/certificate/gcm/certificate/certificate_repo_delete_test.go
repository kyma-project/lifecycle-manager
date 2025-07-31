package certificate_test

import (
	"context"
	"testing"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDelete_ClientCallSucceeds_Returns(t *testing.T) {
	clientStub := &deleteClientStub{}
	certificateRepository, err := gcmcertificate.NewRepository(
		clientStub,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Delete(t.Context(), certName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.object)
	assert.Equal(t, certName, clientStub.object.Name)
	assert.Equal(t, certNamespace, clientStub.object.Namespace)
}

func TestDelete_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &deleteClientStub{
		err: assert.AnError,
	}
	certificateRepository, err := gcmcertificate.NewRepository(
		clientStub,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Delete(t.Context(), certName)

	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to delete certificate")
	assert.True(t, clientStub.called)
}

type deleteClientStub struct {
	client.Client
	called bool
	object *gcertv1alpha1.Certificate
	err    error
}

func (c *deleteClientStub) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	c.called = true
	c.object = obj.(*gcertv1alpha1.Certificate)
	return c.err
}
