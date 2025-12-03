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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCertRepo := &mockCertificateRepository{
				exists:    tt.certExists,
				existsErr: tt.certExistsErr,
			}
			mockSecretRepo := &mockSecretRepository{
				exists:    tt.secretExists,
				existsErr: tt.secretExistsErr,
			}
			useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

			applicable, err := useCase.IsApplicable(context.Background(), tt.kyma)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedApplicable, applicable)
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

		assert.NoError(t, res.Err)
		assert.Equal(t, usecase.DeleteWatcherCertificateSetup, res.UseCase)
		assert.True(t, mockCertRepo.deleteCalled)
		assert.True(t, mockSecretRepo.deleteCalled)
	})

	t.Run("returns error when certificate delete fails", func(t *testing.T) {
		expectedErr := errors.New("certificate delete failed")
		mockCertRepo := &mockCertificateRepository{
			deleteErr: expectedErr,
		}
		mockSecretRepo := &mockSecretRepository{}
		useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

		res := useCase.Execute(context.Background(), kyma)

		assert.Error(t, res.Err)
		assert.ErrorIs(t, res.Err, expectedErr)
		assert.Equal(t, usecase.DeleteWatcherCertificateSetup, res.UseCase)
		assert.True(t, mockCertRepo.deleteCalled)
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

		assert.Error(t, res.Err)
		assert.ErrorIs(t, res.Err, expectedErr)
		assert.Equal(t, usecase.DeleteWatcherCertificateSetup, res.UseCase)
		assert.True(t, mockCertRepo.deleteCalled)
		assert.True(t, mockSecretRepo.deleteCalled)
	})
}

func TestWatcherCertificateCleanup_Name(t *testing.T) {
	mockCertRepo := &mockCertificateRepository{}
	mockSecretRepo := &mockSecretRepository{}
	useCase := usecases.NewWatcherCertificateCleanup(mockCertRepo, mockSecretRepo)

	assert.Equal(t, usecase.DeleteWatcherCertificateSetup, useCase.Name())
}

type mockCertificateRepository struct {
	exists       bool
	existsErr    error
	deleteErr    error
	deleteCalled bool
}

func (m *mockCertificateRepository) Exists(ctx context.Context, name string) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockCertificateRepository) Delete(ctx context.Context, name string) error {
	m.deleteCalled = true
	return m.deleteErr
}

type mockSecretRepository struct {
	exists       bool
	existsErr    error
	deleteErr    error
	deleteCalled bool
}

func (m *mockSecretRepository) Exists(ctx context.Context, name string) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockSecretRepository) Delete(ctx context.Context, name string) error {
	m.deleteCalled = true
	return m.deleteErr
}
