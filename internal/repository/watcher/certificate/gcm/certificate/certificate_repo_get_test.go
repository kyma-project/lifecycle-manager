package certificate_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
)

func TestGetRenewalTime_ClientCallSucceedsAndRenewalTimeParsing_Success(t *testing.T) {
	now := time.Now()
	expected := now.Add(certDuration - certRenewBefore)

	clientStub := &getClientStub{
		object: &gcertv1alpha1.Certificate{
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

	renewalTime, err := certificateRepository.GetRenewalTime(t.Context(), certName)

	require.NoError(t, err)
	// compare as strings as renewalTime lost some ticks during string conversion and parsing
	assert.Equal(t, expected.Format(time.RFC3339), renewalTime.Format(time.RFC3339))
	assert.True(t, clientStub.called)
}

func Test_CertificateClient_GetRenewalTime_Error(t *testing.T) {
	clientStub := &getClientStub{
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

	renewalTime, err := certificateRepository.GetRenewalTime(t.Context(), certName)

	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.called)
}

func Test_CertificateClient_GetRenewalTime_Error_NoExpirationDate(t *testing.T) {
	clientStub := &getClientStub{
		object: &gcertv1alpha1.Certificate{
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

	renewalTime, err := certificateRepository.GetRenewalTime(t.Context(), certName)

	require.ErrorIs(t, err, certificate.ErrNoRenewalTime)
	assert.Contains(t, err.Error(), "no expiration date")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.called)
}

func Test_CertificateClient_GetRenewalTime_Error_InvalidExpirationDate(t *testing.T) {
	clientStub := &getClientStub{
		object: &gcertv1alpha1.Certificate{
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

	renewalTime, err := certificateRepository.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse certificate's expiration date")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.called)
}

func Test_CertificateClient_GetValidity_Success(t *testing.T) {
	expectedNotBefore := time.Now().UTC()
	expectedNotAfter := time.Now().Add(certDuration).UTC()
	certificateStateMessage := fmt.Sprintf(
		"certificate (SN 3A:7F:23:4B:12:98:D4:00:1C:2A:BB:77:AC:E3:F1:54) valid from %v to %v",
		expectedNotBefore,
		expectedNotAfter,
	)

	certificateRepository, err := newCertRepoWithGetClientStubWithStatusMessage(certificateStateMessage)
	require.NoError(t, err)

	notBefore, notAfter, err := certificateRepository.GetValidity(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, expectedNotBefore, notBefore)
	assert.Equal(t, expectedNotAfter, notAfter)
}

func Test_CertificateClient_GetValidity_GetCertificateError(t *testing.T) {
	clientStub := &getClientStub{
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

	renewalTime, err := certificateRepository.GetRenewalTime(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get certificate")
	assert.True(t, renewalTime.IsZero())
	assert.True(t, clientStub.called)
}

func Test_CertificateClient_GetValidity_NilMessageError(t *testing.T) {
	clientStub := &getClientStub{
		object: &gcertv1alpha1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       gcertv1alpha1.CertificateKind,
				APIVersion: gcertv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: gcertv1alpha1.CertificateStatus{
				Message: nil,
			},
		},
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

	notBefore, notAfter, err := certificateRepository.GetValidity(t.Context(), certName)

	require.Error(t, err)
	require.ErrorIs(t, err, gcmcertificate.ErrCertificateStatusNotContainMessage)
	assert.Zero(t, notBefore)
	assert.Zero(t, notAfter)
	assert.True(t, clientStub.called)
}

func Test_CertificateClient_GetValidity_NoValidDatesError(t *testing.T) {
	certificateStateMessage := "certificate (SN 3A:7F:23:4B:12:98:D4:00:1C:2A:BB:77:AC:E3:F1:54) valid"

	certificateRepository, err := newCertRepoWithGetClientStubWithStatusMessage(certificateStateMessage)
	require.NoError(t, err)

	notBefore, notAfter, err := certificateRepository.GetValidity(t.Context(), certName)

	require.Error(t, err)
	require.ErrorIs(t, err, gcmcertificate.ErrInputStringNotContainValidDates)
	assert.Zero(t, notBefore)
	assert.Zero(t, notAfter)
}

func Test_CertificateClient_GetValidity_InvalidNotBeforeDateError(t *testing.T) {
	expectedNotAfter := time.Now().Add(certDuration).UTC()
	certificateStateMessage := fmt.Sprintf(
		"certificate (SN 3A:7F:23:4B:12:98:D4:00:1C:2A:BB:77:AC:E3:F1:54) valid from "+
			"2025-04-24 13:60:60.148938 +0000 UTC to %v",
		expectedNotAfter,
	)

	certificateRepository, err := newCertRepoWithGetClientStubWithStatusMessage(certificateStateMessage)
	require.NoError(t, err)

	notBefore, notAfter, err := certificateRepository.GetValidity(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse notBefore date")
	assert.Zero(t, notBefore)
	assert.Zero(t, notAfter)
}

func Test_CertificateClient_GetValidity_InvalidNotAfterDateError(t *testing.T) {
	expectedNotBefore := time.Now().Add(certDuration).UTC()
	certificateStateMessage := fmt.Sprintf(
		"certificate (SN 3A:7F:23:4B:12:98:D4:00:1C:2A:BB:77:AC:E3:F1:54) valid from %v to "+
			"2025-04-24 13:60:60.148938 +0000 UTC",
		expectedNotBefore,
	)

	certificateRepository, err := newCertRepoWithGetClientStubWithStatusMessage(certificateStateMessage)
	require.NoError(t, err)

	notBefore, notAfter, err := certificateRepository.GetValidity(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse notAfter date")
	assert.Zero(t, notBefore)
	assert.Zero(t, notAfter)
}

type getClientStub struct {
	client.Client

	object *gcertv1alpha1.Certificate
	called bool
	err    error
}

func (c *getClientStub) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.called = true
	if c.object != nil {
		c.object.DeepCopyInto(obj.(*gcertv1alpha1.Certificate))
	}
	return c.err
}

func newCertRepoWithGetClientStubWithStatusMessage(message string) (*gcmcertificate.Repository, error) {
	clientStub := &getClientStub{
		object: &gcertv1alpha1.Certificate{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       gcertv1alpha1.CertificateKind,
				APIVersion: gcertv1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      certName,
				Namespace: certNamespace,
			},
			Status: gcertv1alpha1.CertificateStatus{
				Message: &message,
			},
		},
	}
	certRepo, err := gcmcertificate.NewRepository(
		clientStub,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			Namespace:   certNamespace,
			Duration:    certDuration,
			RenewBefore: certRenewBefore,
			KeySize:     int(certKeySize),
		},
	)

	return certRepo, err
}
