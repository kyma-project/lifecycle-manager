package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ensureFinalizer := usecases.NewEnsureFinalizer(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	shouldExecute, err := ensureFinalizer.ShouldExecute(context.Background(), mrm)
	require.NoError(t, err)
	require.True(t, shouldExecute)

	executeErr := ensureFinalizer.Execute(context.Background(), mrm)
	require.NoError(t, executeErr)
	require.True(t, mockRepo.EnsureFinalizerCalled)
	require.Equal(t, mrm.Name, mockRepo.CalledWithModule)
	require.Equal(t, shared.MandatoryModuleFinalizer, mockRepo.CalledWithFinalizer)
}

func TestEnsureFinalizer_WithFinalizer(t *testing.T) {
	t.Parallel()

	mockRepo := &MockMrmEnsureFinalizerRepo{}
	ensureFinalizer := usecases.NewEnsureFinalizer(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name:       random.Name(),
			Finalizers: []string{shared.MandatoryModuleFinalizer},
		},
	}

	shouldExecute, err := ensureFinalizer.ShouldExecute(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, shouldExecute)
}

func TestEnsureFinalizer_RepositoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("repository error")
	mockRepo := &MockMrmEnsureFinalizerRepo{
		EnsureFinalizerError: expectedErr,
	}
	ensureFinalizer := usecases.NewEnsureFinalizer(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: metav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	executeErr := ensureFinalizer.Execute(context.Background(), mrm)
	require.ErrorIs(t, executeErr, expectedErr)
	require.True(t, mockRepo.EnsureFinalizerCalled)
}
