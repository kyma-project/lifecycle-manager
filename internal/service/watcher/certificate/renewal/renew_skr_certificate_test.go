package renewal_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/renewal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenewSkrCertificate_Succeeds(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{}
	service := renewal.NewService(mockCertRepo, nil, gatewaySecretName)

	err := service.RenewSkrCertificate(t.Context(), certName)

	require.NoError(t, err)
	assert.True(t, mockCertRepo.renewCalled)
	assert.Equal(t, certName, mockCertRepo.receivedCertName)
}

func TestRenewSkrCertificate_Fails(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{
		renewErr: assert.AnError,
	}
	service := renewal.NewService(mockCertRepo, nil, gatewaySecretName)

	err := service.RenewSkrCertificate(t.Context(), certName)

	require.ErrorIs(t, err, mockCertRepo.renewErr)
	assert.True(t, mockCertRepo.renewCalled)
	assert.Equal(t, certName, mockCertRepo.receivedCertName)
}
