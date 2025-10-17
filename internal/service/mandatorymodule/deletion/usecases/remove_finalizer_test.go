package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type MockMrmRemoFinalizerRepo struct {
	RemoveFinalizerCalled bool
	RemoveFinalizerError  error
	CalledWithModule      string
	CalledWithFinalizer   string
}

func (m *MockMrmRemoFinalizerRepo) RemoveFinalizer(_ context.Context, moduleName string, finalizer string) error {
	m.RemoveFinalizerCalled = true
	m.CalledWithModule = moduleName
	m.CalledWithFinalizer = finalizer
	return m.RemoveFinalizerError
}

func TestRemoveFinalizer_WithFinalizer(t *testing.T) {
	t.Parallel()

	mockRepo := &MockMrmRemoFinalizerRepo{}
	removeFinalizer := usecases.NewRemoveFinalizer(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:       random.Name(),
			Finalizers: []string{shared.MandatoryModuleFinalizer},
		},
	}

	isApplicable, err := removeFinalizer.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.True(t, isApplicable)

	executeErr := removeFinalizer.Execute(context.Background(), mrm)
	require.NoError(t, executeErr)
	require.True(t, mockRepo.RemoveFinalizerCalled)
	require.Equal(t, mrm.Name, mockRepo.CalledWithModule)
	require.Equal(t, shared.MandatoryModuleFinalizer, mockRepo.CalledWithFinalizer)
}

func TestRemoveFinalizer_WithoutFinalizer(t *testing.T) {
	t.Parallel()

	mockRepo := &MockMrmRemoFinalizerRepo{}
	removeFinalizer := usecases.NewRemoveFinalizer(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	isApplicable, err := removeFinalizer.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, isApplicable)
}

func TestRemoveFinalizer_RepositoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("repository error")
	mockRepo := &MockMrmRemoFinalizerRepo{
		RemoveFinalizerError: expectedErr,
	}
	removeFinalizer := usecases.NewRemoveFinalizer(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:       random.Name(),
			Finalizers: []string{shared.MandatoryModuleFinalizer},
		},
	}

	executeErr := removeFinalizer.Execute(context.Background(), mrm)
	require.ErrorIs(t, executeErr, expectedErr)
	require.True(t, mockRepo.RemoveFinalizerCalled)
}
