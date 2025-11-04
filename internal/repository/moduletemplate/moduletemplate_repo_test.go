package moduletemplate_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository/moduletemplate"
)

type clientStub struct {
	client.Client

	getCalled    bool
	updateCalled bool
	listCalled   bool
	getErr       error
	updateErr    error
	listErr      error

	capturedNamespace string
	capturedLabels    map[string]string
	capturedFields    map[string]string

	moduleTemplate  *v1beta2.ModuleTemplate
	moduleTemplates []v1beta2.ModuleTemplate
}

func (c *clientStub) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	c.getCalled = true
	if c.moduleTemplate != nil {
		c.moduleTemplate.DeepCopyInto(obj.(*v1beta2.ModuleTemplate))
	}
	return c.getErr
}

func (c *clientStub) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	c.updateCalled = true
	return c.updateErr
}

func (c *clientStub) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.listCalled = true

	for _, opt := range opts {
		if nsOpt, ok := opt.(client.InNamespace); ok {
			c.capturedNamespace = string(nsOpt)
		}
		if labelOpt, ok := opt.(client.MatchingLabels); ok {
			c.capturedLabels = labelOpt
		}
		if fieldOpt, ok := opt.(client.MatchingFields); ok {
			c.capturedFields = fieldOpt
		}
	}

	if c.listErr != nil {
		return c.listErr
	}

	if moduleTemplateList, ok := list.(*v1beta2.ModuleTemplateList); ok {
		moduleTemplateList.Items = c.moduleTemplates
	}

	return nil
}

func TestRepository_EnsureFinalizer(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testModuleTemplateName := "test-moduletemplate"
	testFinalizer := "test-finalizer"

	t.Run("adds finalizer when not present", func(t *testing.T) {
		moduleTemplate := &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testModuleTemplateName,
				Namespace:  testNamespace,
				Finalizers: []string{},
			},
		}

		stub := &clientStub{moduleTemplate: moduleTemplate}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})

	t.Run("does not update when finalizer already present", func(t *testing.T) {
		moduleTemplate := &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testModuleTemplateName,
				Namespace:  testNamespace,
				Finalizers: []string{testFinalizer},
			},
		}

		stub := &clientStub{moduleTemplate: moduleTemplate}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when get fails", func(t *testing.T) {
		expectedErr := errors.New("get error")
		stub := &clientStub{getErr: expectedErr}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.Error(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		moduleTemplate := &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testModuleTemplateName,
				Namespace:  testNamespace,
				Finalizers: []string{},
			},
		}

		expectedErr := errors.New("update error")
		stub := &clientStub{moduleTemplate: moduleTemplate, updateErr: expectedErr}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.EnsureFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to add finalizer to ModuleTemplate")
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})
}

func TestRepository_RemoveFinalizer(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testModuleTemplateName := "test-moduletemplate"
	testFinalizer := "test-finalizer"

	t.Run("removes finalizer when present", func(t *testing.T) {
		moduleTemplate := &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testModuleTemplateName,
				Namespace:  testNamespace,
				Finalizers: []string{testFinalizer},
			},
		}

		stub := &clientStub{moduleTemplate: moduleTemplate}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})

	t.Run("does not update when finalizer not present", func(t *testing.T) {
		moduleTemplate := &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testModuleTemplateName,
				Namespace:  testNamespace,
				Finalizers: []string{},
			},
		}

		stub := &clientStub{moduleTemplate: moduleTemplate}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.NoError(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when get fails", func(t *testing.T) {
		expectedErr := errors.New("get error")
		stub := &clientStub{getErr: expectedErr}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.Error(t, err)
		require.True(t, stub.getCalled)
		require.False(t, stub.updateCalled)
	})

	t.Run("returns error when update fails", func(t *testing.T) {
		moduleTemplate := &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:       testModuleTemplateName,
				Namespace:  testNamespace,
				Finalizers: []string{testFinalizer},
			},
		}

		expectedErr := errors.New("update error")
		stub := &clientStub{moduleTemplate: moduleTemplate, updateErr: expectedErr}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		err := repo.RemoveFinalizer(ctx, testModuleTemplateName, testFinalizer)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to remove finalizer from ModuleTemplate")
		require.True(t, stub.getCalled)
		require.True(t, stub.updateCalled)
	})
}

func TestRepository_Get(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testModuleTemplateName := "test-moduletemplate"

	t.Run("returns ModuleTemplate when successful", func(t *testing.T) {
		expectedModuleTemplate := &v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      testModuleTemplateName,
				Namespace: testNamespace,
			},
		}

		stub := &clientStub{moduleTemplate: expectedModuleTemplate}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		result, err := repo.Get(ctx, testModuleTemplateName)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, testModuleTemplateName, result.Name)
		require.Equal(t, testNamespace, result.Namespace)
		require.True(t, stub.getCalled)
	})

	t.Run("returns error when client get fails", func(t *testing.T) {
		expectedErr := errors.New("client get error")
		stub := &clientStub{getErr: expectedErr}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		result, err := repo.Get(ctx, testModuleTemplateName)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to get ModuleTemplate")
		require.Contains(t, err.Error(), testModuleTemplateName)
		require.Contains(t, err.Error(), testNamespace)
		require.True(t, stub.getCalled)
	})
}

func TestRepository_ListAllForModule(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testModuleName := "test-module"

	t.Run("successfully lists all ModuleTemplates for module", func(t *testing.T) {
		expectedModuleTemplates := []v1beta2.ModuleTemplate{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "template1",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.ModuleName: testModuleName},
				},
			},
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      "template2",
					Namespace: testNamespace,
					Labels:    map[string]string{shared.ModuleName: testModuleName},
				},
			},
		}

		stub := &clientStub{moduleTemplates: expectedModuleTemplates}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		result, err := repo.ListAllForModule(ctx, testModuleName)

		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, expectedModuleTemplates, result)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})

	t.Run("returns empty list when no ModuleTemplates found", func(t *testing.T) {
		stub := &clientStub{moduleTemplates: []v1beta2.ModuleTemplate{}}
		repo := moduletemplate.NewRepository(stub, testNamespace)

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
		repo := moduletemplate.NewRepository(stub, testNamespace)

		result, err := repo.ListAllForModule(ctx, testModuleName)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to list ModuleTemplates for module")
		require.Contains(t, err.Error(), testModuleName)
		require.True(t, stub.listCalled)
		require.Equal(t, testNamespace, stub.capturedNamespace)
		require.Equal(t, testModuleName, stub.capturedLabels[shared.ModuleName])
	})

	t.Run("uses correct module name in label selector", func(t *testing.T) {
		differentModuleName := "different-module-name"
		stub := &clientStub{moduleTemplates: []v1beta2.ModuleTemplate{}}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		_, err := repo.ListAllForModule(ctx, differentModuleName)

		require.NoError(t, err)
		require.True(t, stub.listCalled)
		require.Equal(t, differentModuleName, stub.capturedLabels[shared.ModuleName])
	})
}

func TestRepository_GetSpecificVersionForModule(t *testing.T) {
	ctx := context.Background()
	testNamespace := "test-namespace"
	testModuleName := "test-module"
	testVersion := "1.0.0"

	t.Run("successful call returns single ModuleTemplate", func(t *testing.T) {
		expected := v1beta2.ModuleTemplate{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", testModuleName, testVersion),
				Namespace: testNamespace,
				Labels:    map[string]string{shared.ModuleName: testModuleName},
			},
		}

		stub := &clientStub{moduleTemplate: &expected}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		result, err := repo.GetSpecificVersionForModule(ctx, testModuleName, testVersion)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expected.Name, result.Name)
		require.True(t, stub.getCalled)
	})

	t.Run("get error", func(t *testing.T) {
		expectedErr := errors.New("get error")
		stub := &clientStub{getErr: expectedErr}
		repo := moduletemplate.NewRepository(stub, testNamespace)

		result, err := repo.GetSpecificVersionForModule(ctx, testModuleName, testVersion)

		require.True(t, stub.getCalled)
		require.Error(t, err)
		require.Nil(t, result)
		require.ErrorIs(t, err, expectedErr)
	})
}
