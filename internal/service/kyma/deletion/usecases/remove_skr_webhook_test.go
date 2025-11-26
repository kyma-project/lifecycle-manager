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

type mockSKRWebhookManager struct {
	removeCalled bool
	removeErr    error
}

func (m *mockSKRWebhookManager) Remove(ctx context.Context, kyma *v1beta2.Kyma) error {
	m.removeCalled = true
	return m.removeErr
}

func TestRemoveSkrWebhookUseCase_IsApplicable(t *testing.T) {
	now := apimetav1.Now()
	tests := []struct {
		name               string
		kyma               *v1beta2.Kyma
		expectedApplicable bool
	}{
		{
			name: "applicable when kyma is deleting and state is deleting",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			expectedApplicable: true,
		},
		{
			name: "not applicable when kyma is not being deleted",
			kyma: &v1beta2.Kyma{
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			expectedApplicable: false,
		},
		{
			name: "not applicable when state is not deleting",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateReady,
				},
			},
			expectedApplicable: false,
		},
		{
			name: "not applicable when deletion timestamp is zero",
			kyma: &v1beta2.Kyma{
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			expectedApplicable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &mockSKRWebhookManager{}
			useCase := usecases.NewRemoveSkrWebhookUseCase(mockManager)

			applicable, err := useCase.IsApplicable(context.Background(), tt.kyma)

			require.NoError(t, err)
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

	t.Run("successfully removes webhook", func(t *testing.T) {
		mockManager := &mockSKRWebhookManager{}
		useCase := usecases.NewRemoveSkrWebhookUseCase(mockManager)

		res := useCase.Execute(context.Background(), kyma)

		assert.NoError(t, res.Err)
		assert.Equal(t, usecase.DeleteSkrWatcher, res.UseCase)
		assert.True(t, mockManager.removeCalled)
	})

	t.Run("returns error when remove fails", func(t *testing.T) {
		expectedErr := errors.New("remove failed")
		mockManager := &mockSKRWebhookManager{
			removeErr: expectedErr,
		}
		useCase := usecases.NewRemoveSkrWebhookUseCase(mockManager)

		res := useCase.Execute(context.Background(), kyma)

		assert.Error(t, res.Err)
		assert.Equal(t, expectedErr, res.Err)
		assert.Equal(t, usecase.DeleteSkrWatcher, res.UseCase)
		assert.True(t, mockManager.removeCalled)
	})

	t.Run("skips when webhook manager is nil", func(t *testing.T) {
		useCase := usecases.NewRemoveSkrWebhookUseCase(nil)

		res := useCase.Execute(context.Background(), kyma)

		assert.NoError(t, res.Err)
		assert.Equal(t, usecase.DeleteSkrWatcher, res.UseCase)
	})
}

func TestRemoveSkrWebhookUseCase_Name(t *testing.T) {
	mockManager := &mockSKRWebhookManager{}
	useCase := usecases.NewRemoveSkrWebhookUseCase(mockManager)

	assert.Equal(t, usecase.DeleteSkrWatcher, useCase.Name())
}
