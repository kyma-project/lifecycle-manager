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
			name: "applicable when resources exist",
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
			expectedApplicable: true,
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockRepo := &mockSkrWebhookResourcesRepository{
				resourcesExist:    testCase.resourcesExist,
				resourcesExistErr: testCase.resourcesExistErr,
			}
			usecase := usecases.NewRemoveSkrWebhookResources(mockRepo)

			applicable, err := usecase.IsApplicable(context.Background(), testCase.kyma)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, testCase.expectedApplicable, applicable)
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
		useCase := usecases.NewRemoveSkrWebhookResources(mockRepo)

		res := useCase.Execute(context.Background(), kyma)

		require.NoError(t, res.Err)
		assert.Equal(t, usecase.DeleteSkrWebhookResources, res.UseCase)
		assert.True(t, mockRepo.deleteResourcesCalled)
	})

	t.Run("returns error when webhook resources delete fails", func(t *testing.T) {
		expectedErr := errors.New("delete failed")
		mockRepo := &mockSkrWebhookResourcesRepository{
			deleteResourcesErr: expectedErr,
		}
		useCase := usecases.NewRemoveSkrWebhookResources(mockRepo)

		res := useCase.Execute(context.Background(), kyma)

		require.Error(t, res.Err)
		require.ErrorIs(t, res.Err, expectedErr)
		assert.Equal(t, usecase.DeleteSkrWebhookResources, res.UseCase)
		assert.True(t, mockRepo.deleteResourcesCalled)
	})
}

func TestRemoveSkrWebhookUseCase_Name(t *testing.T) {
	mockRepo := &mockSkrWebhookResourcesRepository{}
	useCase := usecases.NewRemoveSkrWebhookResources(mockRepo)

	assert.Equal(t, usecase.DeleteSkrWebhookResources, useCase.Name())
}

type mockSkrWebhookResourcesRepository struct {
	resourcesExist        bool
	resourcesExistErr     error
	deleteResourcesErr    error
	deleteResourcesCalled bool
}

func (m *mockSkrWebhookResourcesRepository) ResourcesExist(_ context.Context, kymaName string) (bool, error) {
	return m.resourcesExist, m.resourcesExistErr
}

func (m *mockSkrWebhookResourcesRepository) DeleteWebhookResources(_ context.Context, kymaName string) error {
	m.deleteResourcesCalled = true
	return m.deleteResourcesErr
}
