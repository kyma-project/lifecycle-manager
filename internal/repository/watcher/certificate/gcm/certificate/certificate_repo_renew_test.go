package certificate_test

import (
	"context"
	"testing"
	"time"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
)

var truer = true

func TestRenew_Succeeds(t *testing.T) {
	kcpClient := &clientStub{}
	certRepo, _ := certificate.NewRepository(kcpClient,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)

	err := certRepo.Renew(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, certName, kcpClient.getLastCallArg.Name)
	assert.Equal(t, certNamespace, kcpClient.getLastCallArg.Namespace)
	assert.Equal(t, &truer, kcpClient.updateCallArg.Spec.Renew)
	assert.Nil(t, kcpClient.updateCallArg.Spec.EnsureRenewedAfter)
}

func TestRenew_GetCertificateError(t *testing.T) {
	kcpClient := &clientStub{
		getErr: assert.AnError,
	}
	certRepo, _ := certificate.NewRepository(kcpClient,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)

	err := certRepo.Renew(t.Context(), certName)

	require.ErrorIs(t, err, kcpClient.getErr)
	assert.Contains(t, err.Error(), "could not get certificate for renewal")
	assert.Equal(t, 0, kcpClient.updateCalls)
}

func TestRenew_UpdateCertificateError(t *testing.T) {
	kcpClient := &clientStub{
		updateErr: assert.AnError,
	}
	certRepo, _ := certificate.NewRepository(kcpClient,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
			Namespace:   certNamespace,
		},
	)

	err := certRepo.Renew(t.Context(), certName)

	require.ErrorIs(t, err, kcpClient.updateErr)
	assert.Contains(t, err.Error(), "failed to update certificate for renewal")
}

type clientStub struct {
	client.Client

	getErr         error
	getLastCallArg client.ObjectKey
	updateErr      error
	updateCalls    int
	updateCallArg  gcertv1alpha1.Certificate
}

func (c *clientStub) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.getLastCallArg = key
	if c.getErr != nil {
		return c.getErr
	}

	*(obj.(*gcertv1alpha1.Certificate)) = gcertv1alpha1.Certificate{
		Spec: gcertv1alpha1.CertificateSpec{
			EnsureRenewedAfter: &apimetav1.Time{Time: time.Now()},
		},
	}

	return nil
}

func (c *clientStub) Update(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
	c.updateCalls++
	c.updateCallArg = *(obj.(*gcertv1alpha1.Certificate))
	if c.updateErr != nil {
		return c.updateErr
	}
	return nil
}
