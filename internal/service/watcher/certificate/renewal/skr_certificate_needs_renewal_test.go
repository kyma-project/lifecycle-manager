package renewal_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type mockSecretRepository struct {
	getCalled                 bool
	getErr                    error
	gatewaySecretAnnotations  map[string]string
	receivedGatewaySecretName string
}

func (m *mockSecretRepository) Get(_ context.Context, secretName string) (*apicorev1.Secret, error) {
	m.getCalled = true
	m.receivedGatewaySecretName = secretName
	if m.getErr != nil {
		return nil, m.getErr
	}
	return &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: m.gatewaySecretAnnotations,
		},
	}, nil
}

func TestSkrCertificateNeedsRenewal_RenewalNeeded(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	mockSecretRepo := &mockSecretRepository{
		gatewaySecretAnnotations: map[string]string{
			shared.LastModifiedAtAnnotation: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		},
	}
	service := renewal.NewService(mockCertRepo, mockSecretRepo, gatewaySecretName)

	needsRenewal, err := service.SkrCertificateNeedsRenewal(t.Context(), certName)

	require.NoError(t, err)
	assert.True(t, needsRenewal)
	assert.Equal(t, certName, mockCertRepo.receivedCertName)
	assert.Equal(t, gatewaySecretName, mockSecretRepo.receivedGatewaySecretName)
}

func TestSkrCertificateNeedsRenewal_NoRenewalNeeded(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{
		getValidityStart: time.Now().Add(-1 * time.Hour),
		getValidityEnd:   time.Now().Add(3 * time.Hour),
	}
	mockSecretRepo := &mockSecretRepository{
		gatewaySecretAnnotations: map[string]string{
			shared.LastModifiedAtAnnotation: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		},
	}
	service := renewal.NewService(mockCertRepo, mockSecretRepo, gatewaySecretName)

	needsRenewal, err := service.SkrCertificateNeedsRenewal(t.Context(), certName)

	require.NoError(t, err)
	assert.False(t, needsRenewal)
}

func TestSkrCertificateNeedsRenewal_CertificateValidityError(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{
		getValidityErr: assert.AnError,
	}
	mockSecretRepo := &mockSecretRepository{
		gatewaySecretAnnotations: map[string]string{
			shared.LastModifiedAtAnnotation: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		},
	}
	service := renewal.NewService(mockCertRepo, mockSecretRepo, gatewaySecretName)

	needsRenewal, err := service.SkrCertificateNeedsRenewal(t.Context(), certName)

	require.ErrorIs(t, err, mockCertRepo.getValidityErr)
	assert.Contains(t, err.Error(), "failed to determine SKR client certificate validity")
	assert.False(t, needsRenewal)
}

func TestSkrCertificateNeedsRenewal_GatewaySecretError(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	mockSecretRepo := &mockSecretRepository{
		getErr: assert.AnError,
	}
	service := renewal.NewService(mockCertRepo, mockSecretRepo, gatewaySecretName)

	needsRenewal, err := service.SkrCertificateNeedsRenewal(t.Context(), certName)

	require.ErrorIs(t, err, mockSecretRepo.getErr)
	assert.Contains(t, err.Error(), "failed to get gateway secret")
	assert.False(t, needsRenewal)
}

func TestSkrCertificateNeedsRenewal_MissingAnnotation(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{
		getValidityStart: time.Now().Add(-2 * time.Hour),
		getValidityEnd:   time.Now().Add(2 * time.Hour),
	}
	mockSecretRepo := &mockSecretRepository{
		gatewaySecretAnnotations: map[string]string{},
	}
	service := renewal.NewService(mockCertRepo, mockSecretRepo, gatewaySecretName)

	needsRenewal, err := service.SkrCertificateNeedsRenewal(t.Context(), certName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway secret is missing lastModifiedAt annotation")
	assert.False(t, needsRenewal)
}
