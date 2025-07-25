package certmanager_test

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
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate"
)

func TestCreate_WhenCalledAndClientSucceeds_Returns(t *testing.T) {
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
			CommonName:  certCommonNameName,
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
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Create(t.Context(),
		certName,
		certCommonNameName,
		certDNSNames,
	)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.NotNil(t, clientStub.calledArg)
	assert.Equal(t, expectedCertificate, clientStub.calledArg)
}

func TestCreate_WhenCalledAndClientReturnsError_ReturnsError(t *testing.T) {
	clientStub := &patchClientStub{
		err: assert.AnError,
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Create(t.Context(),
		certName,
		certCommonNameName,
		certDNSNames,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to patch certificate")
	assert.True(t, clientStub.called)
}

type patchClientStub struct {
	client.Client
	called    bool
	calledArg *certmanagerv1.Certificate
	err       error
}

func (c *patchClientStub) Patch(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
	c.called = true
	c.calledArg = obj.(*certmanagerv1.Certificate)
	return c.err
}
