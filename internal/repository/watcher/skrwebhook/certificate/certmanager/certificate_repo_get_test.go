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
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/config"
	certerror "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/error"
)

func TestGetRenewalTime_ClientSucceeds_ReturnsRenewalTime(t *testing.T) {
	clientStub := &getClientStub{
		object: &certmanagerv1.Certificate{
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
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, clientStub.object.Status.RenewalTime.Time, renewalTime)
	assert.True(t, clientStub.called)
}

func TestGetRenewalTime_ClientReturnsError_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		err: assert.AnError,
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.called)
}

func TestGetRenewalTime_CertificateContainsNoRenewalTime_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		object: &certmanagerv1.Certificate{
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
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	renewalTime, err := certClient.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Equal(t, certerror.ErrNoRenewalTime, err)
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.called)
}

func TestGetValidity_ClientCallSucceeds_Returns(t *testing.T) {
	clientStub := &getClientStub{
		object: &certmanagerv1.Certificate{
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
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.NoError(t, err)
	assert.Equal(t, clientStub.object.Status.NotBefore.Time, notBefore)
	assert.Equal(t, clientStub.object.Status.NotAfter.Time, notAfter)
	assert.True(t, clientStub.called)
}

func TestGetValidity_CertificateContainsNoNotBefore_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		object: &certmanagerv1.Certificate{
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
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.ErrorIs(t, err, certmanager.ErrNoNotBefore)
	assert.True(t, notBefore.IsZero())
	assert.True(t, notAfter.IsZero())
	assert.True(t, clientStub.called)
}

func TestGetValidity_CertificateContainsNoNotAfter_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		object: &certmanagerv1.Certificate{
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
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.ErrorIs(t, err, certmanager.ErrNoNotAfter)
	assert.True(t, notBefore.IsZero())
	assert.True(t, notAfter.IsZero())
	assert.True(t, clientStub.called)
}

func TestGetValidity_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		err: assert.AnError,
	}
	certClient := certmanager.NewCertificateRepository(
		clientStub,
		issuerName,
		certNamespace,
		config.CertificateValues{
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     certKeySize,
		},
	)

	notBefore, notAfter, err := certClient.GetValidity(t.Context(), certName, certNamespace)

	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.Zero(t, notBefore)
	assert.Zero(t, notAfter)
	assert.True(t, clientStub.called)
}

type getClientStub struct {
	client.Client
	object *certmanagerv1.Certificate
	called bool
	err    error
}

func (c *getClientStub) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.called = true
	if c.object != nil {
		c.object.DeepCopyInto(obj.(*certmanagerv1.Certificate))
	}
	return c.err
}
