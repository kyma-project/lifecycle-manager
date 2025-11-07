package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type MockMrmEnsureFinalizerRepo struct {
	EnsureFinalizerCalled bool
	EnsureFinalizerError  error
	CalledWithModule      string
	CalledWithFinalizer   string
}

func (m *MockMrmEnsureFinalizerRepo) EnsureFinalizer(_ context.Context, moduleName string, finalizer string) error {
	m.EnsureFinalizerCalled = true
	m.CalledWithModule = moduleName
	m.CalledWithFinalizer = finalizer
	return m.EnsureFinalizerError
}

func TestEnsureFinalizer_WithoutFinalizer(t *testing.T) {
	t.Parallel()

	mockRepo := &MockMrmEnsureFinalizerRepo{}
	mockEventHandler := &mockEventHandler{}
	ensureFinalizer := usecases.NewEnsureFinalizer(mockRepo, mockEventHandler)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	isApplicable, err := ensureFinalizer.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.True(t, isApplicable)

	executeErr := ensureFinalizer.Execute(context.Background(), mrm)
	require.NoError(t, executeErr)
	require.True(t, mockRepo.EnsureFinalizerCalled)
	require.Equal(t, mrm.Name, mockRepo.CalledWithModule)
	require.Equal(t, shared.MandatoryModuleFinalizer, mockRepo.CalledWithFinalizer)
	require.False(t, mockEventHandler.Called)
}

func TestEnsureFinalizer_WithoutFinalizer_ButInDeletingState(t *testing.T) {
	t.Parallel()

	mockRepo := &MockMrmEnsureFinalizerRepo{}
	mockEventHandler := &mockEventHandler{}
	ensureFinalizer := usecases.NewEnsureFinalizer(mockRepo, mockEventHandler)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:              random.Name(),
			DeletionTimestamp: &apimetav1.Time{Time: time.Now()},
		},
	}

	isApplicable, err := ensureFinalizer.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, isApplicable)
	require.NoError(t, err)
	require.False(t, isApplicable)
	require.False(t, mockEventHandler.Called)
}

func TestEnsureFinalizer_WithFinalizer(t *testing.T) {
	t.Parallel()

	mockRepo := &MockMrmEnsureFinalizerRepo{}
	mockEventHandler := &mockEventHandler{}
	ensureFinalizer := usecases.NewEnsureFinalizer(mockRepo, mockEventHandler)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:       random.Name(),
			Finalizers: []string{shared.MandatoryModuleFinalizer},
		},
	}

	isApplicable, err := ensureFinalizer.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, isApplicable)
	require.False(t, mockEventHandler.Called)
}

func TestEnsureFinalizer_RepositoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("repository error")
	mockRepo := &MockMrmEnsureFinalizerRepo{
		EnsureFinalizerError: expectedErr,
	}
	mockEventHandler := &mockEventHandler{}
	ensureFinalizer := usecases.NewEnsureFinalizer(mockRepo, mockEventHandler)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	executeErr := ensureFinalizer.Execute(context.Background(), mrm)
	require.ErrorIs(t, executeErr, expectedErr)
	require.True(t, mockRepo.EnsureFinalizerCalled)
	require.True(t, mockEventHandler.Called)
	require.Equal(t, usecases.SettingFinalizerErrorEvent, mockEventHandler.Reason)
}
