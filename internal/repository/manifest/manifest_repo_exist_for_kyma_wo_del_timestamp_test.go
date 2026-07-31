package manifest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	manifestrepo "github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRepository_ExistForKymaWithoutDeletionTimestamp(t *testing.T) {
	ctx := context.Background()
	testNamespace := random.Name()
	testKymaName := random.Name()

	t.Run("returns true when manifests exist without deletion timestamp", func(t *testing.T) {
		expectedMetadata := []apimetav1.PartialObjectMetadata{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "manifest1",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.KymaName: testKymaName},
				},
			},
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "manifest2",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.KymaName: testKymaName},
				},
			},
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "manifest3",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.KymaName: testKymaName},
				},
			},
		}
		stub := &clientStub{partialObjectMetadata: expectedMetadata}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKymaWithoutDeletionTimestamp(ctx, testKymaName)

		require.NoError(t, err)
		require.True(t, exists)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})

	t.Run("returns false when all manifests have deletion timestamp", func(t *testing.T) {
		stub := &clientStub{
			partialObjectMetadata: []apimetav1.PartialObjectMetadata{
				{
					ObjectMeta: apimetav1.ObjectMeta{
						Name:      "manifest1",
						Namespace: testNamespace,
						Labels:    map[string]string{shared.KymaName: testKymaName},
						DeletionTimestamp: &apimetav1.Time{
							Time: time.Now(),
						},
					},
				},
				{
					ObjectMeta: apimetav1.ObjectMeta{
						Name:      "manifest2",
						Namespace: testNamespace,
						Labels:    map[string]string{shared.KymaName: testKymaName},
						DeletionTimestamp: &apimetav1.Time{
							Time: time.Now().Add(5 * time.Minute),
						},
					},
				},
				{
					ObjectMeta: apimetav1.ObjectMeta{
						Name:      "manifest3",
						Namespace: testNamespace,
						Labels:    map[string]string{shared.KymaName: testKymaName},
						DeletionTimestamp: &apimetav1.Time{
							Time: time.Now().Add(10 * time.Minute),
						},
					},
				},
			},
		}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKymaWithoutDeletionTimestamp(ctx, testKymaName)

		require.NoError(t, err)
		require.False(t, exists)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})

	t.Run("returns true when at least one manifest exists without deletion timestamp", func(t *testing.T) {
		stub := &clientStub{
			partialObjectMetadata: []apimetav1.PartialObjectMetadata{
				{
					ObjectMeta: apimetav1.ObjectMeta{
						Name:      "manifest1",
						Namespace: testNamespace,
						Labels:    map[string]string{shared.KymaName: testKymaName},
						DeletionTimestamp: &apimetav1.Time{
							Time: time.Now(),
						},
					},
				},
				{
					ObjectMeta: apimetav1.ObjectMeta{
						Name:      "manifest2",
						Namespace: testNamespace,
						Labels:    map[string]string{shared.KymaName: testKymaName},
					},
				},
				{
					ObjectMeta: apimetav1.ObjectMeta{
						Name:      "manifest3",
						Namespace: testNamespace,
						Labels:    map[string]string{shared.KymaName: testKymaName},
						DeletionTimestamp: &apimetav1.Time{
							Time: time.Now().Add(10 * time.Minute),
						},
					},
				},
			},
		}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKymaWithoutDeletionTimestamp(ctx, testKymaName)

		require.NoError(t, err)
		require.True(t, exists)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})

	t.Run("returns false when no manifests exist for Kyma", func(t *testing.T) {
		stub := &clientStub{partialObjectMetadata: []apimetav1.PartialObjectMetadata{}}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKymaWithoutDeletionTimestamp(ctx, testKymaName)

		require.NoError(t, err)
		require.False(t, exists)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})

	t.Run("returns error when list fails", func(t *testing.T) {
		expectedErr := errors.New("list error")
		stub := &clientStub{listErr: expectedErr}
		repo := manifestrepo.NewRepository(stub, testNamespace)

		exists, err := repo.ExistForKymaWithoutDeletionTimestamp(ctx, testKymaName)

		require.Error(t, err)
		require.False(t, exists)
		require.Contains(t, err.Error(), "failed to list Manifests for kyma")
		require.Contains(t, err.Error(), testKymaName)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testKymaName, stub.capturedLabels[shared.KymaName])
	})
}
