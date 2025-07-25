package certmanager_test

import (
	"context"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate"
)

func TestGetRenewalTime_WhenCalledAndClientSucceeds_ReturnsRenewalTime(t *testing.T) {
	clientStub := &getClientStub{
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

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, clientStub.getCert.Status.RenewalTime.Time, renewalTime)
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_Error(t *testing.T) {
	clientStub := &getClientStub{
		getErr: assert.AnError,
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

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetRenewalTime_NoRenewalTime(t *testing.T) {
	clientStub := &getClientStub{
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

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Equal(t, certificate.ErrNoRenewalTime, err)
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_Success(t *testing.T) {
	clientStub := &getClientStub{
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
				NotBefore: &apimetav1.Time{Time: time.Now().Add(-time.Hour)},
				NotAfter:  &apimetav1.Time{Time: time.Now().Add(time.Hour)},
			},
		},
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

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.Equal(t, clientStub.getCert.Status.NotBefore.Time, notBefore)
	assert.Equal(t, clientStub.getCert.Status.NotAfter.Time, notAfter)
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_NoNotBefore(t *testing.T) {
	clientStub := &getClientStub{
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
				NotAfter: &apimetav1.Time{Time: time.Now().Add(time.Hour)},
			},
		},
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

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Equal(t, certmanager.ErrNoNotBefore, err)
	assert.True(t, notBefore.IsZero())
	assert.True(t, notAfter.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_NoNotAfter(t *testing.T) {
	clientStub := &getClientStub{
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
				NotBefore: &apimetav1.Time{Time: time.Now().Add(-time.Hour)},
			},
		},
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

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Equal(t, certmanager.ErrNoNotAfter, err)
	assert.True(t, notBefore.IsZero())
	assert.True(t, notAfter.IsZero())
	assert.True(t, clientStub.getCalled)
}

func Test_CertificateClient_GetValidity_GetError(t *testing.T) {
	clientStub := &getClientStub{
		getErr: assert.AnError,
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

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.Zero(t, notBefore)
	assert.Zero(t, notAfter)
	assert.True(t, clientStub.getCalled)
}

type getClientStub struct {
	client.Client
	getCert   *certmanagerv1.Certificate
	getCalled bool
	getErr    error
}

func (c *getClientStub) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.getCalled = true
	if c.getCert != nil {
		c.getCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
	}
	return c.getErr
}
