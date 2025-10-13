package manifest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
)

type clientStub struct {
	client.Client

	deleteAllOfCalled bool
	listCalled        bool
	deleteAllOfErr    error
	listErr           error

	capturedNamespace  string
	capturedLabels     map[string]string
	capturedObjectType client.Object

	manifests []v1beta2.Manifest
}

func (c *clientStub) DeleteAllOf(_ context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	c.deleteAllOfCalled = true
	c.capturedObjectType = obj

	for _, opt := range opts {
		if nsOpt, ok := opt.(client.InNamespace); ok {
			c.capturedNamespace = string(nsOpt)
		}
		if labelOpt, ok := opt.(client.MatchingLabels); ok {
			c.capturedLabels = map[string]string(labelOpt)
		}
	}

	return c.deleteAllOfErr
}

func (c *clientStub) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.listCalled = true

	for _, opt := range opts {
		if nsOpt, ok := opt.(client.InNamespace); ok {
			c.capturedNamespace = string(nsOpt)
		}
		if labelOpt, ok := opt.(client.MatchingLabels); ok {
			c.capturedLabels = map[string]string(labelOpt)
		}
	}

	if c.listErr != nil {
		return c.listErr
	}

	if manifestList, ok := list.(*v1beta2.ManifestList); ok {
		manifestList.Items = c.manifests
	}

	return nil
}

func TestRepository_DeleteAllForModule(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testModuleName := "test-module"

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

	t.Run("uses correct module name in label selector", func(t *testing.T) {
		differentModuleName := "different-module-name"
		stub := &clientStub{}
		repo := manifest.NewRepository(stub, testNamespace)

		err := repo.DeleteAllForModule(ctx, differentModuleName)

		require.NoError(t, err)
		require.True(t, stub.deleteAllOfCalled)
		require.Equal(t, differentModuleName, stub.capturedLabels[shared.ModuleName])
	})
}

func TestRepository_ListAllForModule(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testModuleName := "test-module"

	t.Run("successfully lists all manifests for module", func(t *testing.T) {
		expectedManifests := []v1beta2.Manifest{
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

		stub := &clientStub{manifests: expectedManifests}
		repo := manifest.NewRepository(stub, testNamespace)

		result, err := repo.ListAllForModule(ctx, testModuleName)

		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, expectedManifests, result)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})

	t.Run("returns empty list when no manifests found", func(t *testing.T) {
		stub := &clientStub{manifests: []v1beta2.Manifest{}}
		repo := manifest.NewRepository(stub, testNamespace)

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
		repo := manifest.NewRepository(stub, testNamespace)

		result, err := repo.ListAllForModule(ctx, testModuleName)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to list Manifests for module")
		require.Contains(t, err.Error(), testModuleName)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})

	t.Run("uses correct module name in label selector", func(t *testing.T) {
		differentModuleName := "another-module-name"
		stub := &clientStub{manifests: []v1beta2.Manifest{}}
		repo := manifest.NewRepository(stub, testNamespace)

		_, err := repo.ListAllForModule(ctx, differentModuleName)

		require.NoError(t, err)
		require.True(t, stub.listCalled)
		require.Equal(t, differentModuleName, stub.capturedLabels[shared.ModuleName])
	})
}
