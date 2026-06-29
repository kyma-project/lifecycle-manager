package render_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
)

func Test_defaultTransforms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		render.ResourceTransform

		name      string
		resources []*unstructured.Unstructured
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			render.DisclaimerTransform,
			"empty DisclaimerTransform",
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...any) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			render.KymaComponentTransform,
			"empty KymaComponentTransform",
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...any) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			render.ManagedByOwnedBy,
			"empty WatchedByManagedByOwnedBy",
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...any) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			render.DisclaimerTransform,
			"simple DisclaimerTransform",
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...any) bool {
				require.NoError(t, err)
				unstructs, ok := i[0].([]*unstructured.Unstructured)
				assert.True(testingT, ok)
				unstruct := unstructs[0]
				assert.NotEmpty(testingT, unstruct)
				assert.NotNil(testingT, unstruct.GetAnnotations())
				assert.Contains(testingT, unstruct.GetAnnotations(), render.DisclaimerAnnotation)
				assert.Equal(testingT, render.DisclaimerAnnotationValue,
					unstruct.GetAnnotations()[render.DisclaimerAnnotation])
				return true
			},
		},
		{
			render.KymaComponentTransform,
			"simple KymaComponentTransform",
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...any) bool {
				require.NoError(t, err)
				unstructs, ok := i[0].([]*unstructured.Unstructured)
				assert.True(testingT, ok)
				unstruct := unstructs[0]
				assert.NotEmpty(testingT, unstruct)
				assert.NotNil(testingT, unstruct.GetLabels())
				assert.Contains(testingT, unstruct.GetLabels(), "app.kubernetes.io/component")
				assert.Contains(testingT, unstruct.GetLabels(), "app.kubernetes.io/part-of")
				assert.Equal(testingT, "Kyma", unstruct.GetLabels()["app.kubernetes.io/part-of"])
				return true
			},
		},
		{
			render.ManagedByOwnedBy,
			"simple WatchedByManagedByOwnedBy",
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...any) bool {
				require.NoError(t, err)
				unstructs, ok := i[0].([]*unstructured.Unstructured)
				assert.True(testingT, ok)
				unstruct := unstructs[0]
				assert.NotEmpty(testingT, unstruct)
				assert.NotNil(testingT, unstruct.GetLabels())
				assert.NotNil(testingT, unstruct.GetAnnotations())
				assert.Contains(testingT, unstruct.GetLabels(), shared.ManagedBy)
				assert.Contains(testingT, unstruct.GetAnnotations(), shared.OwnedByAnnotation)
				assert.Equal(testingT, shared.ManagedByLabelValue,
					unstruct.GetLabels()[shared.ManagedBy])
				return true
			},
		},
	}
	for _, testCase := range tests {
		t.Run(
			testCase.name, func(t *testing.T) {
				t.Parallel()
				manifest := &v1beta2.Manifest{}
				manifest.SetName("test-object")
				err := testCase.ResourceTransform(t.Context(), manifest, testCase.resources)
				testCase.wantErr(
					t, err, testCase.resources,
				)
			},
		)
	}
}

func TestGetDefaultResourceTransforms(t *testing.T) {
	t.Parallel()
	transforms := render.GetDefaultResourceTransforms()
	require.Len(t, transforms, 4)
	expected := []uintptr{
		reflect.ValueOf(render.ManagedByOwnedBy).Pointer(),
		reflect.ValueOf(render.KymaComponentTransform).Pointer(),
		reflect.ValueOf(render.DisclaimerTransform).Pointer(),
		reflect.ValueOf(render.DockerImageLocalizationTransform).Pointer(),
	}
	for i, tr := range transforms {
		require.Equal(t, expected[i], reflect.ValueOf(tr).Pointer())
	}
}
