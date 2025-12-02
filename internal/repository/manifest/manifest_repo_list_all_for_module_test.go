package manifest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	manifestrepo "github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRepository_ListAllForModule(t *testing.T) {
	ctx := context.Background()
	testNamespace := random.Name()
	testModuleName := random.Name()

	t.Run("successfully lists all manifests for module", func(t *testing.T) {
		expectedMetadata := []apimetav1.PartialObjectMetadata{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "manifest1",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.ModuleName: testModuleName},
				},
			},
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "manifest2",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.ModuleName: testModuleName},
				},
			},
		}

		stub := &clientStub{partialObjectMetadata: expectedMetadata}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		result, err := repo.ListAllForModule(ctx, testModuleName)

		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, expectedMetadata, result)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})

	t.Run("returns empty list when no manifests found", func(t *testing.T) {
		stub := &clientStub{partialObjectMetadata: []apimetav1.PartialObjectMetadata{}}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		result, err := repo.ListAllForModule(ctx, testModuleName)

		require.NoError(t, err)
		require.Empty(t, result)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})

	t.Run("returns error when list fails", func(t *testing.T) {
		expectedErr := errors.New("list error")
		stub := &clientStub{listErr: expectedErr}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		result, err := repo.ListAllForModule(ctx, testModuleName)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to list Manifests for module")
		require.Contains(t, err.Error(), testModuleName)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})
}
