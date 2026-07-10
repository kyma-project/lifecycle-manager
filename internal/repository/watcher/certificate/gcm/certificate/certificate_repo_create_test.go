package certificate_test

import (
	"context"
	"testing"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gcmcertapplyv1alpha1 "github.com/kyma-project/lifecycle-manager/api/applyconfigurations/gardener/cert/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
)

func TestCreate_ClientCallSucceeds_Returns(t *testing.T) {
	expectedCertificate := gcmcertapplyv1alpha1.Certificate(certName, certNamespace).
		WithSpec(gcmcertapplyv1alpha1.CertificateSpec().
			WithCommonName(certCommonName).
			WithDuration(apimetav1.Duration{Duration: certDuration}).
			WithRenewBefore(apimetav1.Duration{Duration: certRenewBefore}).
			WithDNSNames(certDNSNames...).
			WithSecretName(certName).
			WithSecretLabels(certificate.GetCertificateLabels()).
			WithIssuerRef(gcmcertapplyv1alpha1.IssuerRef().
				WithName(issuerName).
				WithNamespace(issuerNamespace),
			).
			WithPrivateKey(gcmcertapplyv1alpha1.CertificatePrivateKey().
				WithAlgorithm(gcertv1alpha1.RSAKeyAlgorithm).
				WithSize(certKeySize),
			),
		)

	clientStub := &applyClientStub{}
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
	clientStub := &applyClientStub{
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
	assert.Contains(t, err.Error(), "failed to apply certificate")
	assert.True(t, clientStub.called)
}

type applyClientStub struct {
	client.Client

	called bool
	object *gcmcertapplyv1alpha1.CertificateApplyConfiguration
	err    error
}

func (c *applyClientStub) Apply(
	_ context.Context, obj machineryruntime.ApplyConfiguration, _ ...client.ApplyOption,
) error {
	c.called = true
	c.object = obj.(*gcmcertapplyv1alpha1.CertificateApplyConfiguration)
	return c.err
}
