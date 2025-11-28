package manifest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRepository_ExistForKyma(t *testing.T) {
	ctx := context.Background()
	testNamespace := random.Name()
	testKymaName := random.Name()

	t.Run("returns true when manifests exist for Kyma", func(t *testing.T) {
		expectedMetadata := []apimetav1.PartialObjectMetadata{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "manifest1",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.KymaName: testKymaName},
				},
			},
		}

		stub := &clientStub{partialObjectMetadata: expectedMetadata}
		repo := manifest.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKyma(ctx, testKymaName)

		require.NoError(t, err)
		require.True(t, exists)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})

	t.Run("returns false when no manifests exist for Kyma", func(t *testing.T) {
		stub := &clientStub{partialObjectMetadata: []apimetav1.PartialObjectMetadata{}}
		repo := manifest.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKyma(ctx, testKymaName)

		require.NoError(t, err)
		require.False(t, exists)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})

	t.Run("returns error when list fails", func(t *testing.T) {
		expectedErr := errors.New("list error")
		stub := &clientStub{listErr: expectedErr}
		repo := manifest.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKyma(ctx, testKymaName)

		require.Error(t, err)
		require.False(t, exists)
		require.Contains(t, err.Error(), "failed to list Manifests for kyma")
		require.Contains(t, err.Error(), testKymaName)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})
}
