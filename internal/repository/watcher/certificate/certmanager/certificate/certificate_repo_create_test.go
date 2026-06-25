package certificate_test

import (
	"context"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagerapplyv1 "github.com/cert-manager/cert-manager/pkg/client/applyconfigurations/certmanager/v1"
	certmanagerapplymetav1 "github.com/cert-manager/cert-manager/pkg/client/applyconfigurations/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate"
	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
)

func TestCreate_ClientCallSucceeds_Returns(t *testing.T) {
	expectedCertApply := certmanagerapplyv1.Certificate(certName, certNamespace).
		WithSpec(certmanagerapplyv1.CertificateSpec().
			WithCommonName(certCommonNameName).
			WithSubject(certmanagerapplyv1.X509Subject().
				WithOrganizationalUnits(certificate.DefaultOrganizationalUnit).
				WithOrganizations(certificate.DefaultOrganization).
				WithLocalities(certificate.DefaultLocality).
				WithProvinces(certificate.DefaultProvince).
				WithCountries(certificate.DefaultCountry),
			).
			WithDuration(apimetav1.Duration{Duration: certDuration}).
			WithRenewBefore(apimetav1.Duration{Duration: certRenewBefore}).
			WithDNSNames(certDNSNames...).
			WithSecretName(certName).
			WithSecretTemplate(certmanagerapplyv1.CertificateSecretTemplate().
				WithLabels(certificate.GetCertificateLabels()),
			).
			WithIssuerRef(certmanagerapplymetav1.IssuerReference().
				WithName(issuerName).
				WithKind(certmanagerv1.IssuerKind),
			).
			WithIsCA(false).
			WithUsages(
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
			).
			WithPrivateKey(certmanagerapplyv1.CertificatePrivateKey().
				WithRotationPolicy(certmanagerv1.RotationPolicyAlways).
				WithEncoding(certmanagerv1.PKCS1).
				WithAlgorithm(certmanagerv1.RSAKeyAlgorithm).
				WithSize(certKeySize),
			),
		)

	clientStub := &applyClientStub{}
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

	err = certificateRepository.Create(t.Context(),
		certName,
		certCommonNameName,
		certDNSNames,
	)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.appliedConfig)
	assert.Equal(t, expectedCertApply, clientStub.appliedConfig)
}

func TestCreate_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &applyClientStub{
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

	err = certificateRepository.Create(t.Context(),
		certName,
		certCommonNameName,
		certDNSNames,
	)

	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}

type applyClientStub struct {
	client.Client

	called        bool
	appliedConfig *certmanagerapplyv1.CertificateApplyConfiguration
	err           error
}

func (c *applyClientStub) Apply(
	_ context.Context, obj machineryruntime.ApplyConfiguration, _ ...client.ApplyOption,
) error {
	c.called = true
	c.appliedConfig = obj.(*certmanagerapplyv1.CertificateApplyConfiguration)
	return c.err
}
