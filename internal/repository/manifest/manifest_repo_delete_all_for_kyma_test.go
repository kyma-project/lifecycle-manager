package manifest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	manifestrepo "github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRepository_DeleteAllForKyma(t *testing.T) {
	ctx := context.Background()
	testNamespace := random.Name()
	testKymaName := random.Name()

	t.Run("successfully deletes all manifests for Kyma", func(t *testing.T) {
		stub := &clientStub{}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		err := repo.DeleteAllForKyma(ctx, testKymaName)

		require.NoError(t, err)
		require.True(t, stub.deleteAllOfCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
		require.IsType(t, &v1beta2.Manifest{}, stub.capturedObjectType)
	})

	t.Run("returns error when deleteAllOf fails", func(t *testing.T) {
		expectedErr := errors.New("delete error")
		stub := &clientStub{deleteAllOfErr: expectedErr}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		err := repo.DeleteAllForKyma(ctx, testKymaName)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to delete all manifests for kyma")
		require.Contains(t, err.Error(), testKymaName)
		require.True(t, stub.deleteAllOfCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})
}
