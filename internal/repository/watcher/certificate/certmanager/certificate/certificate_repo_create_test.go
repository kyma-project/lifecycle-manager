package certificate_test

import (
	"context"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
)

func TestCreate_ClientCallSucceeds_Returns(t *testing.T) {
	expectedCertificate := &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      certName,
			Namespace: certNamespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			CommonName: certCommonNameName,
			Subject: &certmanagerv1.X509Subject{
				OrganizationalUnits: []string{"BTP Kyma Runtime"},
				Organizations:       []string{"SAP SE"},
				Localities:          []string{"Walldorf"},
				Provinces:           []string{"Baden-Württemberg"},
				Countries:           []string{"DE"},
			},
			Duration:    &apimetav1.Duration{Duration: certDuration},
			RenewBefore: &apimetav1.Duration{Duration: certRenewBefore},
			DNSNames:    certDNSNames,
			SecretName:  certName,
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: k8slabels.Set{
					shared.PurposeLabel: shared.CertManager,
					shared.ManagedBy:    shared.OperatorName,
				},
			},
			IssuerRef: certmanagermetav1.ObjectReference{
				Name: issuerName,
				Kind: certmanagerv1.IssuerKind,
			},
			IsCA: false,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				RotationPolicy: certmanagerv1.RotationPolicyAlways,
				Encoding:       certmanagerv1.PKCS1,
				Algorithm:      certmanagerv1.RSAKeyAlgorithm,
				Size:           certKeySize,
			},
		},
	}
	clientStub := &patchClientStub{}
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
	assert.NotNil(t, clientStub.object)
	assert.Equal(t, expectedCertificate, clientStub.object)
}

func TestCreate_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &patchClientStub{
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

type patchClientStub struct {
	client.Client

	called bool
	object *certmanagerv1.Certificate
	err    error
}

func (c *patchClientStub) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
	c.called = true
	c.object = obj.(*certmanagerv1.Certificate)
	return c.err
}
