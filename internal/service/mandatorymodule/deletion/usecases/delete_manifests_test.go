package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

type MockManifestRepo struct {
	ListAllForModuleCalled   bool
	DeleteAllForModuleCalled bool
	ListAllForModuleError    error
	DeleteAllForModuleError  error
	CalledWithModuleName     string
	ManifestsToReturn        []apimetav1.PartialObjectMetadata
}

func (m *MockManifestRepo) ListAllForModule(_ context.Context, moduleName string) (
	[]apimetav1.PartialObjectMetadata, error,
) {
	m.ListAllForModuleCalled = true
	m.CalledWithModuleName = moduleName
	return m.ManifestsToReturn, m.ListAllForModuleError
}

func (m *MockManifestRepo) DeleteAllForModule(_ context.Context, moduleName string) error {
	m.DeleteAllForModuleCalled = true
	m.CalledWithModuleName = moduleName
	return m.DeleteAllForModuleError
}

func TestDeleteManifests_WithManifests(t *testing.T) {
	t.Parallel()

	mockRepo := &MockManifestRepo{
		ManifestsToReturn: []apimetav1.PartialObjectMetadata{
			{ObjectMeta: apimetav1.ObjectMeta{Name: random.Name()}},
		},
	}
	deleteManifests := usecases.NewDeleteManifests(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	isApplicable, err := deleteManifests.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.True(t, isApplicable)
	require.True(t, mockRepo.ListAllForModuleCalled)
	require.Equal(t, mrm.Name, mockRepo.CalledWithModuleName)

	executeErr := deleteManifests.Execute(context.Background(), mrm)
	require.NoError(t, executeErr)
	require.True(t, mockRepo.DeleteAllForModuleCalled)
}

func TestDeleteManifests_NoManifests(t *testing.T) {
	t.Parallel()

	mockRepo := &MockManifestRepo{
		ManifestsToReturn: []apimetav1.PartialObjectMetadata{},
	}
	deleteManifests := usecases.NewDeleteManifests(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	isApplicable, err := deleteManifests.IsApplicable(context.Background(), mrm)
	require.NoError(t, err)
	require.False(t, isApplicable)
}

func TestDeleteManifests_ListError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("list error")
	mockRepo := &MockManifestRepo{
		ListAllForModuleError: expectedErr,
	}
	deleteManifests := usecases.NewDeleteManifests(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	isApplicable, err := deleteManifests.IsApplicable(context.Background(), mrm)
	require.Error(t, err)
	require.False(t, isApplicable)
	require.Contains(t, err.Error(), "failed to list manifests for module")
}

func TestDeleteManifests_DeleteError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("delete error")
	mockRepo := &MockManifestRepo{
		DeleteAllForModuleError: expectedErr,
	}
	deleteManifests := usecases.NewDeleteManifests(mockRepo)
	mrm := &v1beta2.ModuleReleaseMeta{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: random.Name(),
		},
	}

	executeErr := deleteManifests.Execute(context.Background(), mrm)
	require.Error(t, executeErr)
	require.Contains(t, executeErr.Error(), "failed to delete manifests for module")
}
