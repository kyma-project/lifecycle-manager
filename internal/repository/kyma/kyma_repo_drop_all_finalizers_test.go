package kyma_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestRepository_DropAllFinalizers(t *testing.T) {
	testNamespace := random.Name()
	testKymaName := random.Name()

	t.Run("successfully drops all finalizers for Kyma", func(t *testing.T) {
		stub := &clientStub{}
		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropAllFinalizers(t.Context(), testKymaName)

		require.NoError(t, err)
		assert.True(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedPom.GetNamespace())
		assert.Equal(t, testKymaName, stub.capturedPom.GetName())
		assert.Empty(t, stub.capturedPom.GetFinalizers())
		assert.Equal(t, v1beta2.GroupVersion.String(), stub.capturedPom.APIVersion)
		assert.Equal(t, string(shared.KymaKind), stub.capturedPom.GroupVersionKind().Kind)
		// ensure no SSA is used
		assert.Equal(t, client.Merge.Type(), stub.capturedPatch.Type())
	})

	t.Run("returns error when patch fails", func(t *testing.T) {
		stub := &clientStub{patchErr: assert.AnError}
		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropAllFinalizers(t.Context(), testKymaName)

		require.ErrorIs(t, err, assert.AnError)
		assert.True(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedPom.GetNamespace())
		assert.Equal(t, testKymaName, stub.capturedPom.GetName())
	})

	t.Run("ignores not found error", func(t *testing.T) {
		stub := &clientStub{
			patchErr: apierrors.NewNotFound(
				schema.GroupResource{
					Group:    v1beta2.GroupVersion.Group,
					Resource: string(shared.KymaKind),
				},
				random.Name(),
			),
		}
		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropAllFinalizers(t.Context(), testKymaName)

		require.NoError(t, err)
		assert.True(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedPom.GetNamespace())
		assert.Equal(t, testKymaName, stub.capturedPom.GetName())
	})
}

type clientStub struct {
	client.Client

	patchCalled   bool
	patchErr      error
	capturedPom   *apimetav1.PartialObjectMetadata
	capturedPatch client.Patch
}

func (c *clientStub) Patch(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
	c.patchCalled = true

	c.capturedPatch = patch

	if pom, ok := obj.(*apimetav1.PartialObjectMetadata); ok {
		c.capturedPom = pom
	}

	return c.patchErr
}
