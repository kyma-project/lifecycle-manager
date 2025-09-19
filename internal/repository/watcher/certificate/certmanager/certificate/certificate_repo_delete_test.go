package certificate_test

import (
	"context"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
)

func TestDelete_WhenCalledAndClientCallSucceeds_Returns(t *testing.T) {
	clientStub := &deleteClientStub{}
	certificateRepository, err := certmanagercertificate.NewRepository(
		clientStub,
		issuerName,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Delete(t.Context(), certName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.calledArg)
	assert.Equal(t, certName, clientStub.calledArg.Name)
	assert.Equal(t, certNamespace, clientStub.calledArg.Namespace)
}

func TestDelete_WhenCalledAndClientReturnsNotFoundError_IgnoresItAndReturns(t *testing.T) {
	clientStub := &deleteClientStub{
		err: apierrors.NewNotFound(certmanagerv1.Resource("certificates"), certName),
	}
	certificateRepository, err := certmanagercertificate.NewRepository(
		clientStub,
		issuerName,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Delete(t.Context(), certName)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.calledArg)
	assert.Equal(t, certName, clientStub.calledArg.Name)
	assert.Equal(t, certNamespace, clientStub.calledArg.Namespace)
}

func TestDelete_WhenCalledAndClientReturnsOtherError_ReturnsError(t *testing.T) {
	clientStub := &deleteClientStub{
		err: assert.AnError,
	}
	certificateRepository, err := certmanagercertificate.NewRepository(
		clientStub,
		issuerName,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
			Namespace:   certNamespace,
		},
	)
	require.NoError(t, err)

	err = certificateRepository.Delete(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete certificate")
	assert.True(t, clientStub.called)
}

type deleteClientStub struct {
	client.Client

	called    bool
	calledArg *certmanagerv1.Certificate
	err       error
}

func (c *deleteClientStub) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	c.called = true
	c.calledArg = obj.(*certmanagerv1.Certificate)
	return c.err
}
