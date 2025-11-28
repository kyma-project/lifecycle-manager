package manifest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/require"
)

func TestRepository_DeleteAllForModule(t *testing.T) {
	ctx := context.Background()
	testNamespace := random.Name()
	testModuleName := random.Name()

	t.Run("successfully deletes all manifests for module", func(t *testing.T) {
		stub := &clientStub{}
		repo := manifest.NewRepository(stub, testNamespace)

		err := repo.DeleteAllForModule(ctx, testModuleName)

		require.NoError(t, err)
		require.True(t, stub.deleteAllOfCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
		require.IsType(t, &v1beta2.Manifest{}, stub.capturedObjectType)
	})

	t.Run("returns error when deleteAllOf fails", func(t *testing.T) {
		expectedErr := errors.New("delete error")
		stub := &clientStub{deleteAllOfErr: expectedErr}
		repo := manifest.NewRepository(stub, testNamespace)

		err := repo.DeleteAllForModule(ctx, testModuleName)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to delete all manifests for module")
		require.Contains(t, err.Error(), testModuleName)
		require.True(t, stub.deleteAllOfCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})
}
