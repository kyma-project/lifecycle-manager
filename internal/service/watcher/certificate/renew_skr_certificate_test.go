package certificate_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
)

func TestRenewSkrCertificate_WhenNeedsRenew_CallsRenewalServiceRenew(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.CaAddedToBundleAtAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	certService := certificate.NewService(certRepo,
		secretRepo,
		certificate.Config{
			GatewaySecretName: gatewaySecretName,
		},
	)

	err := certService.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, certRepo.getValidityCalled)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, gatewaySecretName, secretRepo.receivedSecretName)
	assert.True(t, certRepo.renewCalled)
	assert.Equal(t, kymaName+expectedCertNameSuffix, certRepo.receivedCertName)
}

func TestRenewSkrCertificate_WhenSecretIndicatesNoRenew_RenewalServiceRenewIsNotCalled(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.CaAddedToBundleAtAnnotation: time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
					},
				},
			},
		},
	}

	certService := certificate.NewService(certRepo,
		secretRepo,
		certificate.Config{
			GatewaySecretName: gatewaySecretName,
		},
	)

	err := certService.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, certRepo.getValidityCalled)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, gatewaySecretName, secretRepo.receivedSecretName)
	assert.False(t, certRepo.renewCalled)
}

func TestRenewSkrCertificate_WhenCertRepoGetValidityReturnsError_RenewalServiceRenewIsNotCalled(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityErr: assert.AnError,
	}
	secretRepo := &secretRepoStub{}

	certService := certificate.NewService(certRepo,
		secretRepo,
		certificate.Config{
			GatewaySecretName: gatewaySecretName,
		},
	)

	err := certService.RenewSkrCertificate(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to determine SKR client certificate validity")
	assert.True(t, certRepo.getValidityCalled)
	assert.False(t, secretRepo.getCalled)
	assert.False(t, certRepo.renewCalled)
}

func TestRenewSkrCertificate_WhenSecretRepoGetReturnsError_RenewalServiceRenewIsNotCalled(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	secretRepo := &secretRepoStub{
		getErrors: []error{assert.AnError},
	}

	certService := certificate.NewService(certRepo,
		secretRepo,
		certificate.Config{
			GatewaySecretName: gatewaySecretName,
		},
	)

	err := certService.RenewSkrCertificate(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to get gateway secret")
	assert.Contains(t, err.Error(), "failed to determine gateway secret caAddedToBundleAt")
	assert.True(t, certRepo.getValidityCalled)
	assert.True(t, secretRepo.getCalled)
	assert.False(t, certRepo.renewCalled)
}

func TestRenewSkrCertificate_WhenCaBundleAnnotation_RenewalServiceRenewIsNotCalled(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{{}},
	}

	certService := certificate.NewService(certRepo,
		secretRepo,
		certificate.Config{
			GatewaySecretName: gatewaySecretName,
		},
	)

	err := certService.RenewSkrCertificate(t.Context(), kymaName)

	require.ErrorIs(t, err, certificate.ErrGatewaySecretMissingBundlingTimeAnnotation)
	assert.Contains(t, err.Error(), "failed to determine gateway secret caAddedToBundleAt")
	assert.True(t, certRepo.getValidityCalled)
	assert.True(t, secretRepo.getCalled)
	assert.False(t, certRepo.renewCalled)
}

func TestRenewSkrCertificate_WhenCaBundleAnnotationFailsToParse_RenewalServiceRenewIsNotCalled(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.CaAddedToBundleAtAnnotation: "not a valid time string",
					},
				},
			},
		},
	}

	certService := certificate.NewService(certRepo,
		secretRepo,
		certificate.Config{
			GatewaySecretName: gatewaySecretName,
		},
	)

	err := certService.RenewSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine gateway secret caAddedToBundleAt")
	assert.Contains(t, err.Error(), "failed to parse gateway secret caAddedToBundleAt annotation")
	assert.True(t, certRepo.getValidityCalled)
	assert.True(t, secretRepo.getCalled)
	assert.False(t, certRepo.renewCalled)
}

func TestRenewSkrCertificate_WhenRenewReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
		renewErr:         assert.AnError,
	}
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						shared.CaAddedToBundleAtAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	certService := certificate.NewService(certRepo,
		secretRepo,
		certificate.Config{
			GatewaySecretName: gatewaySecretName,
		},
	)

	err := certService.RenewSkrCertificate(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "failed to renew SKR certificate")
	assert.True(t, certRepo.getValidityCalled)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, gatewaySecretName, secretRepo.receivedSecretName)
	assert.True(t, certRepo.renewCalled)
	assert.Equal(t, kymaName+expectedCertNameSuffix, certRepo.receivedCertName)
}

func (r *certRepoStub) Renew(_ context.Context, certName string) error {
	r.renewCalled = true
	r.receivedCertName = certName

	if r.renewErr != nil {
		return r.renewErr
	}

	return nil
}

func (r *certRepoStub) GetValidity(_ context.Context, certName string) (time.Time, time.Time, error) {
	r.getValidityCalled = true
	r.receivedCertName = certName
	return r.getValidityStart, r.getValidityEnd, r.getValidityErr
}
