package gardener_test

import (
	"context"
	"math"
	"testing"
	"time"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/gardener"
)

var rsaKeyAlgorithm = gcertv1alpha1.RSAKeyAlgorithm

var (
	issuerName                                   = random.Name()
	issuerNamespace                              = random.Name()
	certName                                     = random.Name()
	certNamespace                                = random.Name()
	certCommonName                               = random.Name()
	certDNSNames                                 = []string{random.Name(), random.Name()}
	certDuration                                 = 24 * time.Hour
	certRenewBefore                              = 12 * time.Hour
	certKeySize     gcertv1alpha1.PrivateKeySize = 4096
)

func Test_CertificateClient_New_Error(t *testing.T) {
	invalidKeySize := math.MaxInt32 + 1
	certClient, err := gardener.NewCertificateClient(
		&kcpClientStub{},
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     invalidKeySize,
		},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, gardener.ErrKeySizeOutOfRange)
	assert.Nil(t, certClient)
}

func Test_CertificateClient_Create_Success(t *testing.T) {
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
			Duration:     &apimetav1.Duration{Duration: certDuration},
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

	clientStub := &kcpClientStub{}
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	err = certClient.Create(t.Context(),
		certName,
		certNamespace,
		certCommonName,
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
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	err = certClient.Create(t.Context(),
		certName,
		certNamespace,
		certCommonName,
		certDNSNames,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to patch certificate")
	assert.True(t, clientStub.patchCalled)
}

func Test_CertificateClient_Delete_Success(t *testing.T) {
	clientStub := &kcpClientStub{}
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	err = certClient.Delete(t.Context(), certName, certNamespace)

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
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	err = certClient.Delete(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete certificate")
	assert.True(t, clientStub.deleteCalled)
}

func Test_CertificateClient_GetRenewalTime_Success(t *testing.T) {
	now := time.Now()
	expected := now.Add(certDuration - certRenewBefore)

	clientStub := &kcpClientStub{
		getCert: &gcertv1alpha1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       gcertv1alpha1.CertificateKind,
				APIVersion: gcertv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: gcertv1alpha1.CertificateStatus{
				ExpirationDate: stringPtr(now.Add(certDuration).Format(time.RFC3339)),
			},
		},
	}
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	// compare as strings as renewalTime lost some ticks during string conversion and parsing
	assert.Equal(t, expected.Format(time.RFC3339), renewalTime.Format(time.RFC3339))
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_Error(t *testing.T) {
	clientStub := &kcpClientStub{
		getErr: assert.AnError,
	}
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_Error_NoExpirationDate(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &gcertv1alpha1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       gcertv1alpha1.CertificateKind,
				APIVersion: gcertv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: gcertv1alpha1.CertificateStatus{},
		},
	}
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.Error(t, err)
	require.ErrorIs(t, err, certificate.ErrNoRenewalTime)
	assert.Contains(t, err.Error(), "no expiration date")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_Error_InvalidExpirationDate(t *testing.T) {
	clientStub := &kcpClientStub{
		getCert: &gcertv1alpha1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       gcertv1alpha1.CertificateKind,
				APIVersion: gcertv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: gcertv1alpha1.CertificateStatus{
				ExpirationDate: &issuerName,
			},
		},
	}
	certClient, err := gardener.NewCertificateClient(
		clientStub,
		issuerName,
		issuerNamespace,
		certificate.CertificateConfig{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)
	require.NoError(t, err)

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse certificate's expiration date")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.getCalled)
}

// Helper functions and stubs

type kcpClientStub struct {
	getCert      *gcertv1alpha1.Certificate
	getCalled    bool
	getErr       error
	deleteCalled bool
	deleteErr    error
	deleteArg    *gcertv1alpha1.Certificate
	patchCalled  bool
	patchErr     error
	patchArg     *gcertv1alpha1.Certificate
}

func (c *kcpClientStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	c.getCalled = true
	if c.getCert != nil {
		//nolint:forcetypeassert // this is a stub
		c.getCert.DeepCopyInto(obj.(*gcertv1alpha1.Certificate))
	}
	return c.getErr
}

func (c *kcpClientStub) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCalled = true
	//nolint:forcetypeassert // this is a stub
	c.deleteArg = obj.(*gcertv1alpha1.Certificate)
	return c.deleteErr
}

func (c *kcpClientStub) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patchCalled = true
	//nolint:forcetypeassert // this is a stub
	c.patchArg = obj.(*gcertv1alpha1.Certificate)
	return c.patchErr
}

func stringPtr(s string) *string {
	return &s
}
