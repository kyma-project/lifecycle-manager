package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
)

func TestWatcherCertificateCleanup_IsApplicable(t *testing.T) {
	now := apimetav1.Now()
	tests := []struct {
		name               string
		kyma               *v1beta2.Kyma
		certExists         bool
		certExistsErr      error
		secretExists       bool
		secretExistsErr    error
		expectedApplicable bool
		expectError        bool
		expectedCertName   string
		expectedSecretName string
	}{
		{
			name: "applicable when certificate exists",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			certExists:         true,
			secretExists:       false,
			expectedApplicable: true,
			expectError:        false,
			expectedCertName:   "test-kyma-webhook-tls",
		},
		{
			name: "applicable when only secret exists",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			certExists:         false,
			secretExists:       true,
			expectedApplicable: true,
			expectError:        false,
			expectedCertName:   "test-kyma-webhook-tls",
			expectedSecretName: "test-kyma-webhook-tls",
		},
		{
			name: "applicable when both certificate and secret exist",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			certExists:         true,
			secretExists:       true,
			expectedApplicable: true,
			expectError:        false,
			expectedCertName:   "test-kyma-webhook-tls",
			expectedSecretName: "", // secret repo is not called because cert repo already leads to IsApplicable
		},
		{
			name: "not applicable when neither certificate nor secret exist",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			certExists:         false,
			secretExists:       false,
			expectedApplicable: false,
			expectError:        false,
			expectedCertName:   "test-kyma-webhook-tls",
			expectedSecretName: "test-kyma-webhook-tls",
		},
		{
			name: "not applicable when kyma is not being deleted",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: "test-kyma",
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			certExists:         true,
			secretExists:       false,
			expectedApplicable: false,
			expectError:        false,
		},
		{
			name: "not applicable when state is not deleting",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateReady,
				},
			},
			certExists:         true,
			secretExists:       false,
			expectedApplicable: false,
			expectError:        false,
		},
		{
			name: "error when certificate Exists fails",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			certExists:         false,
			certExistsErr:      errors.New("failed to check certificate"),
			secretExists:       false,
			expectedApplicable: false,
			expectError:        true,
			expectedCertName:   "test-kyma-webhook-tls",
		},
		{
			name: "error when secret Exists fails",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			certExists:         false,
			secretExists:       false,
			secretExistsErr:    errors.New("failed to check secret"),
			expectedApplicable: false,
			expectError:        true,
			expectedCertName:   "test-kyma-webhook-tls",
			expectedSecretName: "test-kyma-webhook-tls",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockCertRepo := &mockCertificateRepository{
				exists:    testCase.certExists,
				existsErr: testCase.certExistsErr,
			}
			mockSecretRepo := &mockSecretRepository{
				exists:    testCase.secretExists,
				existsErr: testCase.secretExistsErr,
			}
			useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

			applicable, err := useCase.IsApplicable(context.Background(), testCase.kyma)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, testCase.expectedApplicable, applicable)
			if testCase.expectedCertName != "" {
				assert.Equal(t, testCase.expectedCertName, mockCertRepo.receivedName)
			}
			if testCase.expectedSecretName != "" {
				assert.Equal(t, testCase.expectedSecretName, mockSecretRepo.receivedName)
			}
		})
	}
}

func TestWatcherCertificateCleanup_Execute(t *testing.T) {
	now := apimetav1.Now()
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              "test-kyma",
			DeletionTimestamp: &now,
		},
		Status: v1beta2.KymaStatus{
			State: shared.StateDeleting,
		},
	}

	t.Run("successfully deletes certificate and secret", func(t *testing.T) {
		mockCertRepo := &mockCertificateRepository{}
		mockSecretRepo := &mockSecretRepository{}
		useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

		res := useCase.Execute(context.Background(), kyma)

		require.NoError(t, res.Err)
		assert.Equal(t, usecase.DeleteWatcherCertificateSetup, res.UseCase)
		assert.True(t, mockCertRepo.deleteCalled)
		assert.True(t, mockSecretRepo.deleteCalled)
		assert.Equal(t, "test-kyma-webhook-tls", mockCertRepo.receivedName)
		assert.Equal(t, "test-kyma-webhook-tls", mockSecretRepo.receivedName)
	})

	t.Run("returns error when certificate delete fails", func(t *testing.T) {
		expectedErr := errors.New("certificate delete failed")
		mockCertRepo := &mockCertificateRepository{
			deleteErr: expectedErr,
		}
		mockSecretRepo := &mockSecretRepository{}
		useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

		res := useCase.Execute(context.Background(), kyma)

		require.Error(t, res.Err)
		require.ErrorIs(t, res.Err, expectedErr)
		assert.Equal(t, usecase.DeleteWatcherCertificateSetup, res.UseCase)
		assert.True(t, mockCertRepo.deleteCalled)
		assert.Equal(t, "test-kyma-webhook-tls", mockCertRepo.receivedName)
		assert.False(t, mockSecretRepo.deleteCalled) // Should not be called if cert deletion fails
	})

	t.Run("returns error when secret delete fails", func(t *testing.T) {
		expectedErr := errors.New("secret delete failed")
		mockCertRepo := &mockCertificateRepository{}
		mockSecretRepo := &mockSecretRepository{
			deleteErr: expectedErr,
		}
		useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

		res := useCase.Execute(context.Background(), kyma)

		require.Error(t, res.Err)
		require.ErrorIs(t, res.Err, expectedErr)
		assert.Equal(t, usecase.DeleteWatcherCertificateSetup, res.UseCase)
		assert.True(t, mockCertRepo.deleteCalled)
		assert.Equal(t, "test-kyma-webhook-tls", mockCertRepo.receivedName)
		assert.True(t, mockSecretRepo.deleteCalled)
		assert.Equal(t, "test-kyma-webhook-tls", mockSecretRepo.receivedName)
	})
}

func TestWatcherCertificateCleanup_Name(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{}
	mockSecretRepo := &mockSecretRepository{}
	useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

	assert.Equal(t, usecase.DeleteWatcherCertificateSetup, useCase.Name())
}

type mockCertificateRepository struct {
	certificate.CertificateRepository

	exists       bool
	existsErr    error
	deleteErr    error
	deleteCalled bool

	receivedName string
}

func (m *mockCertificateRepository) Exists(_ context.Context, name string) (bool, error) {
	m.receivedName = name
	return m.exists, m.existsErr
}

func (m *mockCertificateRepository) Delete(_ context.Context, name string) error {
	m.receivedName = name
	m.deleteCalled = true
	return m.deleteErr
}

type mockSecretRepository struct {
	exists       bool
	existsErr    error
	deleteErr    error
	deleteCalled bool

	receivedName string
}

func (m *mockSecretRepository) Exists(_ context.Context, name string) (bool, error) {
	m.receivedName = name
	return m.exists, m.existsErr
}

func (m *mockSecretRepository) Delete(_ context.Context, name string) error {
	m.receivedName = name
	m.deleteCalled = true
	return m.deleteErr
}
