package v2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

type testObj struct{ *unstructured.Unstructured }

func (t testObj) GetStatus() shared.Status { panic("status not supported in test object") }
func (t testObj) SetStatus(shared.Status)  { panic("status not supported in test object") }

func Test_defaultTransforms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		declarativev2.ObjectTransform

		name      string
		resources []*unstructured.Unstructured
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			declarativev2.DisclaimerTransform,
			"empty DisclaimerTransform",
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			declarativev2.KymaComponentTransform,
			"empty KymaComponentTransform",
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			declarativev2.ManagedByOwnedBy,
			"empty WatchedByManagedByOwnedBy",
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			declarativev2.DisclaimerTransform,
			"simple DisclaimerTransform",
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				unstructs, ok := i[0].([]*unstructured.Unstructured)
				assert.True(testingT, ok)
				unstruct := unstructs[0]
				assert.NotEmpty(testingT, unstruct)
				assert.NotNil(testingT, unstruct.GetAnnotations())
				assert.Contains(testingT, unstruct.GetAnnotations(), declarativev2.DisclaimerAnnotation)
				assert.Equal(testingT, declarativev2.DisclaimerAnnotationValue,
					unstruct.GetAnnotations()[declarativev2.DisclaimerAnnotation])
				return true
			},
		},
		{
			declarativev2.KymaComponentTransform,
			"simple KymaComponentTransform",
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
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
			declarativev2.ManagedByOwnedBy,
			"simple WatchedByManagedByOwnedBy",
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
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
				obj := &testObj{Unstructured: &unstructured.Unstructured{}}
				obj.SetName("test-object")
				err := testCase.ObjectTransform(t.Context(), obj, testCase.resources)
				testCase.wantErr(
					t, err, testCase.resources,
				)
			},
		)
	}
}
