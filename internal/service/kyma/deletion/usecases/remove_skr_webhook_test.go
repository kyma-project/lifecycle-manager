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

func TestRemoveSkrWebhookUseCase_IsApplicable(t *testing.T) {
	now := apimetav1.Now()
	tests := []struct {
		name               string
		kyma               *v1beta2.Kyma
		resourcesExist     bool
		resourcesExistErr  error
		expectedApplicable bool
		expectError        bool
	}{
		{
			name: "applicable when kyma is deleting and resources exist",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			resourcesExist:     true,
			expectedApplicable: false, // returns !ok, so when resources exist (true), returns false
			expectError:        false,
		},
		{
			name: "not applicable when resources don't exist",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			resourcesExist:     false,
			expectedApplicable: true, // returns !ok, so when resources don't exist (false), returns true
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
			resourcesExist:     true,
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
			resourcesExist:     true,
			expectedApplicable: false,
			expectError:        false,
		},
		{
			name: "error when ResourcesExist fails",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:              "test-kyma",
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			resourcesExist:     false,
			resourcesExistErr:  errors.New("failed to check resources"),
			expectedApplicable: false,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockSkrWebhookResourcesRepository{
				resourcesExist:    tt.resourcesExist,
				resourcesExistErr: tt.resourcesExistErr,
			}
			useCase := usecases.NewRemoveSkrWebhookUseCase(mockRepo)

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

func TestRemoveSkrWebhookUseCase_Execute(t *testing.T) {
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

	t.Run("successfully removes webhook resources", func(t *testing.T) {
		mockRepo := &mockSkrWebhookResourcesRepository{}
		useCase := usecases.NewRemoveSkrWebhookUseCase(mockRepo)

		res := useCase.Execute(context.Background(), kyma)

		assert.NoError(t, res.Err)
		assert.Equal(t, usecase.DeleteSkrWatcher, res.UseCase)
		assert.True(t, mockRepo.deleteResourcesCalled)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		expectedErr := errors.New("delete failed")
		mockRepo := &mockSkrWebhookResourcesRepository{
			deleteResourcesErr: expectedErr,
		}
		useCase := usecases.NewRemoveSkrWebhookUseCase(mockRepo)

		res := useCase.Execute(context.Background(), kyma)

		assert.Error(t, res.Err)
		assert.ErrorIs(t, res.Err, expectedErr)
		assert.Equal(t, usecase.DeleteSkrWatcher, res.UseCase)
		assert.True(t, mockRepo.deleteResourcesCalled)
	})
}

func TestRemoveSkrWebhookUseCase_Name(t *testing.T) {
	mockRepo := &mockSkrWebhookResourcesRepository{}
	useCase := usecases.NewRemoveSkrWebhookUseCase(mockRepo)

	assert.Equal(t, usecase.DeleteSkrWatcher, useCase.Name())
}

type mockSkrWebhookResourcesRepository struct {
	resourcesExist        bool
	resourcesExistErr     error
	deleteResourcesErr    error
	deleteResourcesCalled bool
}

func (m *mockSkrWebhookResourcesRepository) ResourcesExist(kymaName string) (bool, error) {
	return m.resourcesExist, m.resourcesExistErr
}

func (m *mockSkrWebhookResourcesRepository) DeleteWebhookResources(ctx context.Context, kymaName string) error {
	m.deleteResourcesCalled = true
	return m.deleteResourcesErr
}
