package gcm_test

import (
	"context"
	"testing"
	"time"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal/gcm"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRenew_WhenRepoReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		getErr: assert.AnError,
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "could not get certificate for renewal")
	assert.Equal(t, 1, certRepo.getCalls)
	assert.Equal(t, certName, certRepo.getLastCallArg)
	assert.Equal(t, 0, certRepo.updateCalls)
}

func TestRenew_WhenRepoReturnsNil_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.Error(t, err)
	require.ErrorContains(t, err, "could not get certificate for renewal")
	assert.Equal(t, 1, certRepo.getCalls)
	assert.Equal(t, certName, certRepo.getLastCallArg)
	assert.Equal(t, 0, certRepo.updateCalls)
}

func TestRenew_WhenRepoReturnsCert_CallsRepoUpdateWithSpecRenewTrue(t *testing.T) {
	certRepo := &certRepoStub{
		getReturnValue: &gcertv1alpha1.Certificate{
			Spec: gcertv1alpha1.CertificateSpec{
				EnsureRenewedAfter: nil,
				Renew:              nil,
			},
		},
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, 1, certRepo.getCalls)
	assert.Equal(t, certName, certRepo.getLastCallArg)
	assert.Equal(t, 1, certRepo.updateCalls)
	assert.True(t, *certRepo.getReturnValue.Spec.Renew)
	assert.Nil(t, certRepo.getReturnValue.Spec.EnsureRenewedAfter)
}

func TestRenew_WhenRepoReturnsCert_CallsRepoUpdateWithSpecEnsureRenewedAfterNil(t *testing.T) {
	certRepo := &certRepoStub{
		getReturnValue: &gcertv1alpha1.Certificate{
			Spec: gcertv1alpha1.CertificateSpec{
				EnsureRenewedAfter: &apimetav1.Time{},
				Renew:              nil,
			},
		},
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.NoError(t, err)
	assert.Equal(t, 1, certRepo.getCalls)
	assert.Equal(t, certName, certRepo.getLastCallArg)
	assert.Equal(t, 1, certRepo.updateCalls)
	assert.True(t, *certRepo.getReturnValue.Spec.Renew)
	assert.Nil(t, certRepo.getReturnValue.Spec.EnsureRenewedAfter)
}

func TestRenew_WhenRepoUpdateReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		getReturnValue: &gcertv1alpha1.Certificate{
			Spec: gcertv1alpha1.CertificateSpec{},
		},
		updateErr: assert.AnError,
	}
	service := gcm.NewService(certRepo)
	certName := random.Name()

	err := service.Renew(t.Context(), certName)

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "failed to update certificate for renewal")
}

func TestSkrSecretNeedsRenewal_WhenSkrRequestedAtOlderThanGatewayLastModified_ReturnsTrue(t *testing.T) {
	renewalService := gcm.NewService(nil)
	gatewaySecret := &apicorev1.Secret{ // gateway secret, modified now
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: time.Now().Format(time.RFC3339),
			},
		},
	}
	skrSecret := &apicorev1.Secret{ // skr secret, created a minute ago
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.GCMSecretAnnotation: time.Now().Add(-time.Minute).Format(time.RFC3339),
			},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

func TestSkrSecretNeedsRenewal_WhenSkrCreationNewerThanGatewayLastModified_ReturnsFalse(t *testing.T) {
	renewalService := gcm.NewService(nil)
	gatewaySecret := &apicorev1.Secret{ // gateway secret, modified a minute ago
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: time.Now().Add(-time.Minute).Format(time.RFC3339),
			},
		},
	}
	skrSecret := &apicorev1.Secret{ // skr secret, created now
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.GCMSecretAnnotation: time.Now().Format(time.RFC3339),
			},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.False(t, result)
}

func TestSkrSecretNeedsRenewal_GatewaySecretHasNoLastModified_ReturnsTrue(t *testing.T) {
	renewalService := gcm.NewService(nil)
	gatewaySecret := &apicorev1.Secret{ // gateway secret, no lastModifiedAt
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}
	skrSecret := &apicorev1.Secret{}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

func TestRenewSkrCertificate_WhenGatewaySecretHasInvalidLastModified_CallsRenewalServiceRenew(t *testing.T) {
	renewalService := gcm.NewService(nil)
	gatewaySecret := &apicorev1.Secret{ // gateway secret, invalid lastModifiedAt
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: "not a time",
			},
		},
	}
	skrSecret := &apicorev1.Secret{}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

func TestSkrSecretNeedsRenewal_WhenSkrSecretHasNoRequestedAt_ReturnsTrue(t *testing.T) {
	renewalService := gcm.NewService(nil)
	gatewaySecret := &apicorev1.Secret{}
	skrSecret := &apicorev1.Secret{ // skr secret, no requestedAt
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

func TestRenewSkrCertificate_WhenSkrSecretHasInvalidRequestedAt_ReturnsTrue(t *testing.T) {
	renewalService := gcm.NewService(nil)
	gatewaySecret := &apicorev1.Secret{}
	skrSecret := &apicorev1.Secret{ // skr secret, invalid requestedAt
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	result := renewalService.SkrSecretNeedsRenewal(gatewaySecret, skrSecret)

	assert.True(t, result)
}

type certRepoStub struct {
	getCalls          int
	getLastCallArg    string
	getErr            error
	getReturnValue    *gcertv1alpha1.Certificate
	updateCalls       int
	updateLastCallArg *gcertv1alpha1.Certificate
	updateErr         error
}

func (c *certRepoStub) Get(_ context.Context, name string) (*gcertv1alpha1.Certificate, error) {
	c.getCalls++
	c.getLastCallArg = name
	if c.getErr != nil {
		return nil, c.getErr
	}
	return c.getReturnValue, nil
}

func (c *certRepoStub) Update(_ context.Context, cert *gcertv1alpha1.Certificate) error {
	c.updateCalls++
	c.updateLastCallArg = cert
	if c.updateErr != nil {
		return c.updateErr
	}
	return nil
}
