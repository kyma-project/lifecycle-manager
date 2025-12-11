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

	t.Run("successfully drops Kyma finalizer", func(t *testing.T) {
		finalizers := []string{random.Name(), random.Name()}
		stub := &clientStub{
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Finalizers: append(finalizers, shared.KymaFinalizer),
				},
			},
		}

		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropKymaFinalizer(t.Context(), testKymaName)

		require.NoError(t, err)
		assert.True(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedObjectKey.Namespace)
		assert.Equal(t, testKymaName, stub.capturedObjectKey.Name)
		assert.ElementsMatch(t, stub.capturedKyma.GetFinalizers(), finalizers)
		// ensure no SSA is used
		assert.Equal(t, client.Merge.Type(), stub.capturedPatch.Type())
	})

	t.Run("no-op when no Kyma finalizer", func(t *testing.T) {
		finalizers := []string{random.Name(), random.Name()}
		stub := &clientStub{
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Finalizers: finalizers,
				},
			},
		}

		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropKymaFinalizer(t.Context(), testKymaName)

		require.NoError(t, err)
		assert.False(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedObjectKey.Namespace)
		assert.Equal(t, testKymaName, stub.capturedObjectKey.Name)
	})

	t.Run("no-op when empty finalizers", func(t *testing.T) {
		finalizers := []string{}
		stub := &clientStub{
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Finalizers: finalizers,
				},
			},
		}

		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropKymaFinalizer(t.Context(), testKymaName)

		require.NoError(t, err)
		assert.False(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedObjectKey.Namespace)
		assert.Equal(t, testKymaName, stub.capturedObjectKey.Name)
	})

	t.Run("returns error when get Kyma fails", func(t *testing.T) {
		stub := &clientStub{getErr: assert.AnError}
		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropKymaFinalizer(t.Context(), testKymaName)

		require.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "failed to get current finalizers")
		assert.False(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedObjectKey.Namespace)
		assert.Equal(t, testKymaName, stub.capturedObjectKey.Name)
	})

	t.Run("no-op when Kyma not found", func(t *testing.T) {
		stub := &clientStub{
			getErr: apierrors.NewNotFound(
				schema.GroupResource{
					Group:    v1beta2.GroupVersion.Group,
					Resource: string(shared.KymaKind),
				},
				random.Name(),
			),
		}
		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropKymaFinalizer(t.Context(), testKymaName)

		require.NoError(t, err)
		assert.False(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedObjectKey.Namespace)
		assert.Equal(t, testKymaName, stub.capturedObjectKey.Name)
	})

	t.Run("returns error when patch fails", func(t *testing.T) {
		finalizers := []string{random.Name(), random.Name()}
		stub := &clientStub{
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Finalizers: append(finalizers, shared.KymaFinalizer),
				},
			},
			patchErr: assert.AnError,
		}

		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropKymaFinalizer(t.Context(), testKymaName)

		require.ErrorIs(t, err, assert.AnError)
		assert.True(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedObjectKey.Namespace)
		assert.Equal(t, testKymaName, stub.capturedObjectKey.Name)
	})

	t.Run("ignores not found error on patch", func(t *testing.T) {
		finalizers := []string{random.Name(), random.Name()}
		stub := &clientStub{
			kyma: &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Finalizers: append(finalizers, shared.KymaFinalizer),
				},
			},
			patchErr: apierrors.NewNotFound(
				schema.GroupResource{
					Group:    v1beta2.GroupVersion.Group,
					Resource: string(shared.KymaKind),
				},
				random.Name(),
			),
		}

		repo := kymarepo.NewRepository(stub, testNamespace)

		err := repo.DropKymaFinalizer(t.Context(), testKymaName)

		require.NoError(t, err)
		assert.True(t, stub.patchCalled)
		assert.Equal(t, testNamespace, stub.capturedObjectKey.Namespace)
		assert.Equal(t, testKymaName, stub.capturedObjectKey.Name)
	})
}

type clientStub struct {
	client.Client

	kyma *v1beta2.Kyma

	getErr            error
	capturedObjectKey client.ObjectKey

	patchCalled   bool
	patchErr      error
	capturedKyma  *v1beta2.Kyma
	capturedPatch client.Patch
}

func (c *clientStub) Get(_ context.Context, objKey client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if c.kyma != nil {
		c.kyma.DeepCopyInto(obj.(*v1beta2.Kyma))
	}

	c.capturedObjectKey = objKey

	return c.getErr
}

func (c *clientStub) Patch(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
	c.patchCalled = true

	c.capturedPatch = patch

	if kyma, ok := obj.(*v1beta2.Kyma); ok {
		c.capturedKyma = kyma
	}

	return c.patchErr
}
