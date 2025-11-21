package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
)

func TestSetKymaStatusDeletingUseCase_IsApplicable(t *testing.T) {
	tests := []struct {
		name           string
		kyma           *v1beta2.Kyma
		expectedResult bool
		expectedError  error
	}{
		{
			name: "should be applicable when kyma has deletion timestamp and state is not deleting",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateReady,
				},
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "should not be applicable when kyma has no deletion timestamp",
			kyma: &v1beta2.Kyma{
				Status: v1beta2.KymaStatus{
					State: shared.StateReady,
				},
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "should not be applicable when kyma state is already deleting",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
				},
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "should not be applicable when kyma has no deletion timestamp and state is deleting",
			kyma: &v1beta2.Kyma{
				Status: v1beta2.KymaStatus{
					State: shared.StateDeleting,
				},
			},
			expectedResult: false,
			expectedError:  nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockRepo := &kymaStatusRepositoryStub{}
			useCase := usecases.NewSetKymaStatusDeletingUseCase(mockRepo)

			applicable, err := useCase.IsApplicable(context.Background(), testCase.kyma)

			assert.Equal(t, testCase.expectedResult, applicable)
			assert.Equal(t, testCase.expectedError, err)
		})
	}
}

func TestSetKymaStatusDeletingUseCase_Execute(t *testing.T) {
	tests := []struct {
		name           string
		kyma           *v1beta2.Kyma
		repoError      error
		expectedResult result.Result
	}{
		{
			name: "should execute successfully when repository update succeeds",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "test-kyma",
					Namespace: "test-namespace",
				},
			},
			repoError: nil,
			expectedResult: result.Result{
				UseCase: usecase.SetKcpKymaStateDeleting,
				Err:     nil,
			},
		},
		{
			name: "should return error when repository update fails",
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "test-kyma",
					Namespace: "test-namespace",
				},
			},
			repoError: errors.New("repository update failed"),
			expectedResult: result.Result{
				UseCase: usecase.SetKcpKymaStateDeleting,
				Err:     errors.New("repository update failed"),
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// TODO: re-align whether to use mocks or stubs
			mockRepo := &kymaStatusRepositoryStub{}
			mockRepo.On("UpdateStatusDeleting",
				mock.Anything,
				testCase.kyma).Return(testCase.repoError)

			useCase := usecases.NewSetKymaStatusDeletingUseCase(mockRepo)

			res := useCase.Execute(context.Background(), testCase.kyma)

			assert.Equal(t, testCase.expectedResult.UseCase, res.UseCase)
			if testCase.expectedResult.Err != nil {
				require.Error(t, res.Err)
				assert.Equal(t, testCase.expectedResult.Err.Error(), res.Err.Error())
			} else {
				require.NoError(t, res.Err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestSetKymaStatusDeletingUseCase_Execute_VerifyRepositoryCall(t *testing.T) {
	mockRepo := &kymaStatusRepositoryStub{}
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "test-kyma",
			Namespace: "test-namespace",
		},
	}

	mockRepo.On("UpdateStatusDeleting",
		mock.Anything,
		kyma).Return(nil)

	useCase := usecases.NewSetKymaStatusDeletingUseCase(mockRepo)

	useCase.Execute(context.Background(), kyma)

	// Verify the repository was called with the correct parameters
	mockRepo.AssertCalled(t, "UpdateStatusDeleting",
		mock.Anything,
		kyma)
}

type kymaStatusRepositoryStub struct {
	mock.Mock
}

func (m *kymaStatusRepositoryStub) UpdateStatusDeleting(ctx context.Context,
	kyma *v1beta2.Kyma,
) error {
	args := m.Called(ctx, kyma)
	return args.Error(0)
}
