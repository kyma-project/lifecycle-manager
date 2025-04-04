package certmanager_test

import (
	"context"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/certmanager"
)

var (
	issuerName    = random.Name()
	certName      = random.Name()
	certNamespace = random.Name()
	certCAName    = random.Name()
	certDNSNames  = []string{
		random.Name(),
		random.Name(),
	}
	certDuration    = 24 * time.Hour
	certRenewBefore = 12 * time.Hour
	certKeySize     = 4096
)

func Test_CertificateClient_Create_Success(t *testing.T) {
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

	clientStub := &kcpClientStub{}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Create(t.Context(),
		certName,
		certNamespace,
		certCAName,
		certDNSNames,
	)

	require.NoError(t, err)
	assert.True(t, clientStub.patchCalled)
	assert.NotNil(t, clientStub.patchArg)
	assert.Equal(t, expectedCertificate, clientStub.patchArg)
}

func Test_CertificateClient_Create_Error(t *testing.T) {
	clientStub := &kcpClientStub{
		patchErr: assert.AnError,
	}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Create(t.Context(),
		certName,
		certNamespace,
		certCAName,
		certDNSNames,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to patch certificate")
	assert.True(t, clientStub.patchCalled)
}

func Test_CertificateClient_Delete_Success(t *testing.T) {
	clientStub := &kcpClientStub{}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Delete(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.True(t, clientStub.deleteCalled)
	assert.NotNil(t, clientStub.deleteArg)
	assert.Equal(t, certName, clientStub.deleteArg.Name)
	assert.Equal(t, certNamespace, clientStub.deleteArg.Namespace)
}

func Test_CertificateClient_Delete_Error(t *testing.T) {
	clientStub := &kcpClientStub{
		deleteErr: assert.AnError,
	}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Delete(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete certificate")
	assert.True(t, clientStub.deleteCalled)
}

func Test_CertificateClient_Delete_IgnoreNotFoundError(t *testing.T) {
	clientStub := &kcpClientStub{
		deleteErr: apierrors.NewNotFound(certmanagerv1.Resource("certificates"), certName),
	}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	err := certClient.Delete(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.True(t, clientStub.deleteCalled)
	assert.NotNil(t, clientStub.deleteArg)
	assert.Equal(t, certName, clientStub.deleteArg.Name)
	assert.Equal(t, certNamespace, clientStub.deleteArg.Namespace)
}

func Test_CertificateClient_GetRenewalTime_Success(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &certmanagerv1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       certmanagerv1.CertificateKind,
				APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: certmanagerv1.CertificateStatus{
				RenewalTime: &apimetav1.Time{Time: time.Now().Add(-time.Hour)},
			},
		},
	}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.Equal(t, clientStub.getCert.Status.RenewalTime.Time, time)
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_Error(t *testing.T) {
	clientStub := &kcpClientStub{
		getErr: assert.AnError,
	}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, time.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_NoRenewalTime(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &certmanagerv1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       certmanagerv1.CertificateKind,
				APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: certmanagerv1.CertificateStatus{},
		},
	}
	certClient := certmanager.NewCertificateClient(
		clientStub,
		issuerName,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	time, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Equal(t, certmanager.ErrNoRenewalTime, err)
	assert.True(t, time.IsZero())
	assert.True(t, clientStub.getCalled)
}

// Test stubs

type kcpClientStub struct {
	getCert      *certmanagerv1.Certificate
	getCalled    bool
	getErr       error
	deleteCalled bool
	deleteErr    error
	deleteArg    *certmanagerv1.Certificate
	patchCalled  bool
	patchErr     error
	patchArg     *certmanagerv1.Certificate
}

func (c *kcpClientStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCalled = true
	if c.getCert != nil {
		//nolint:forcetypeassert // test code
		c.getCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
	}
	return c.getErr
}

func (c *kcpClientStub) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalled = true
	//nolint:forcetypeassert // test code
	c.deleteArg = obj.(*certmanagerv1.Certificate)
	return c.deleteErr
}

func (c *kcpClientStub) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patchCalled = true
	//nolint:forcetypeassert // test code
	c.patchArg = obj.(*certmanagerv1.Certificate)
	return c.patchErr
}
