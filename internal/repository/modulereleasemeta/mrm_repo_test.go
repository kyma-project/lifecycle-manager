package modulereleasemeta_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
)

type clientStub struct {
	client.Client

	getCalled bool
	getErr    error

	updateCalled bool
	updateErr    error

	listCalled                   bool
	listCalledWithMatchingFields map[string]string
	listErr                      error
	mrm                          *v1beta2.ModuleReleaseMeta
}

func (c *clientStub) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.getCalled = true
	if c.mrm != nil {
		c.mrm.DeepCopyInto(obj.(*v1beta2.ModuleReleaseMeta))
	}
	return c.getErr
}

func (c *clientStub) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	c.updateCalled = true
	return c.updateErr
}

func (c *clientStub) List(_ context.Context, _ client.ObjectList, listOptions ...client.ListOption) error {
	c.listCalled = true
	for _, option := range listOptions {
		if matchingFields, ok := option.(client.MatchingFields); ok {
			c.listCalledWithMatchingFields = matchingFields
		}
	}
	return c.listErr
}

func TestRepository_EnsureFinalizer(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testMRMName := "test-mrm"
	testFinalizer := "test-finalizer"

	t.Run("adds finalizer when not present", func(t *testing.T) {
		mrm := &v1beta2.ModuleReleaseMeta{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testMRMName,
				Namespace:  testNamespace,
				Finalizers: []string{},
			},
		}

		stub := &clientStub{mrm: mrm}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testMRMName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})

	t.Run("does not update when finalizer already present", func(t *testing.T) {
		mrm := &v1beta2.ModuleReleaseMeta{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testMRMName,
				Namespace:  testNamespace,
				Finalizers: []string{testFinalizer},
			},
		}

		stub := &clientStub{mrm: mrm}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testMRMName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when get fails", func(t *testing.T) {
		expectedErr := errors.New("get error")
		stub := &clientStub{getErr: expectedErr}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testMRMName, testFinalizer)

		require.Error(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		mrm := &v1beta2.ModuleReleaseMeta{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testMRMName,
				Namespace:  testNamespace,
				Finalizers: []string{},
			},
		}

		expectedErr := errors.New("update error")
		stub := &clientStub{mrm: mrm, updateErr: expectedErr}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testMRMName, testFinalizer)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to add finalizer to ModuleReleaseMeta")
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})
}

func TestRepository_RemoveFinalizer(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testMRMName := "test-mrm"
	testFinalizer := "test-finalizer"

	t.Run("removes finalizer when present", func(t *testing.T) {
		mrm := &v1beta2.ModuleReleaseMeta{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testMRMName,
				Namespace:  testNamespace,
				Finalizers: []string{testFinalizer},
			},
		}

		stub := &clientStub{mrm: mrm}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testMRMName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})

	t.Run("does not update when finalizer not present", func(t *testing.T) {
		mrm := &v1beta2.ModuleReleaseMeta{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testMRMName,
				Namespace:  testNamespace,
				Finalizers: []string{},
			},
		}

		stub := &clientStub{mrm: mrm}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testMRMName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when get fails", func(t *testing.T) {
		expectedErr := errors.New("get error")
		stub := &clientStub{getErr: expectedErr}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testMRMName, testFinalizer)

		require.Error(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		mrm := &v1beta2.ModuleReleaseMeta{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testMRMName,
				Namespace:  testNamespace,
				Finalizers: []string{testFinalizer},
			},
		}

		expectedErr := errors.New("update error")
		stub := &clientStub{mrm: mrm, updateErr: expectedErr}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testMRMName, testFinalizer)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to remove finalizer from ModuleReleaseMeta")
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})
}

func TestRepository_Get(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testMRMName := "test-mrm"

	t.Run("returns MRM when successful", func(t *testing.T) {
		expectedMRM := &v1beta2.ModuleReleaseMeta{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      testMRMName,
				Namespace: testNamespace,
			},
		}

		stub := &clientStub{mrm: expectedMRM}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		result, err := repo.Get(ctx, testMRMName)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, testMRMName, result.Name)
		require.Equal(t, testNamespace, result.Namespace)
		require.True(t, stub.getCalled)
	})

	t.Run("returns error when client get fails", func(t *testing.T) {
		expectedErr := errors.New("client get error")
		stub := &clientStub{getErr: expectedErr}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		result, err := repo.Get(ctx, testMRMName)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to get ModuleReleaseMeta")
		require.Contains(t, err.Error(), testMRMName)
		require.Contains(t, err.Error(), testNamespace)
		require.True(t, stub.getCalled)
	})
}

func TestRepository_ListMandatory(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"

	t.Run("lists mandatory MRMs successfully", func(t *testing.T) {
		stub := &clientStub{}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		_, err := repo.ListMandatory(ctx)

		require.NoError(t, err)
		require.True(t, stub.listCalled)
		require.Equal(t,
			map[string]string{shared.MrmMandatoryModuleFieldIndexName: shared.MrmMandatoryModuleFieldIndexPositiveValue},
			stub.listCalledWithMatchingFields)
	})

	t.Run("returns error when client list fails", func(t *testing.T) {
		expectedErr := errors.New("client list error")
		stub := &clientStub{listErr: expectedErr}
		repo := modulereleasemeta.NewRepository(stub, testNamespace)

		_, err := repo.ListMandatory(ctx)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to list mandatory ModuleReleaseMeta")
		require.True(t, stub.listCalled)
		require.Equal(t,
			map[string]string{shared.MrmMandatoryModuleFieldIndexName: shared.MrmMandatoryModuleFieldIndexPositiveValue},
			stub.listCalledWithMatchingFields)
	})
}
