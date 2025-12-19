package certificate_test

import (
	"context"
	"testing"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
)

func TestCreate_ClientCallSucceeds_Returns(t *testing.T) {
	expectedCertificate := &gcertv1alpha1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       gcertv1alpha1.CertificateKind,
			APIVersion: gcertv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      certName,
			Namespace: certNamespace,
		},
		Spec: gcertv1alpha1.CertificateSpec{
			CommonName:   &certCommonName,
			Duration:     &apimetav1.Duration{Duration: certDuration},
			RenewBefore:  &apimetav1.Duration{Duration: certRenewBefore},
			DNSNames:     certDNSNames,
			SecretName:   &certName,
			SecretLabels: certificate.GetCertificateLabels(),
			IssuerRef: &gcertv1alpha1.IssuerRef{
				Name:      issuerName,
				Namespace: issuerNamespace,
			},
			PrivateKey: &gcertv1alpha1.CertificatePrivateKey{
				Algorithm: &rsaKeyAlgorithm,
				Size:      &certKeySize,
			},
		},
	}

	clientStub := &patchClientStub{}
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

	err = certificateRepository.Create(t.Context(),
		certName,
		certCommonName,
		certDNSNames,
	)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.object)
	assert.Equal(t, expectedCertificate, clientStub.object)
}

func TestCreate_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &patchClientStub{
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

	err = certificateRepository.Create(t.Context(),
		certName,
		certCommonName,
		certDNSNames,
	)

	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to patch certificate")
	assert.True(t, clientStub.called)
}

type patchClientStub struct {
	client.Client

	called bool
	object *gcertv1alpha1.Certificate
	err    error
}

func (c *patchClientStub) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
	c.called = true
	c.object = obj.(*gcertv1alpha1.Certificate)
	return c.err
}
