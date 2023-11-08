//nolint:testpackage
package v2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type testObj struct{ *unstructured.Unstructured }

func (t testObj) GetStatus() shared.Status { panic("status not supported in test object") }
func (t testObj) SetStatus(shared.Status)  { panic("status not supported in test object") }

//nolint:funlen
func Test_defaultTransforms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ObjectTransform
		resources []*unstructured.Unstructured
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			"empty disclaimerTransform",
			disclaimerTransform,
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			"empty kymaComponentTransform",
			kymaComponentTransform,
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			"empty managedByDeclarativeV2",
			managedByDeclarativeV2,
			[]*unstructured.Unstructured{},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				return true
			},
		},
		{
			"simple disclaimerTransform",
			disclaimerTransform,
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				assert.NoError(testingT, err) //nolint:testifylint
				unstructs, ok := i[0].([]*unstructured.Unstructured)
				assert.True(testingT, ok)
				unstruct := unstructs[0]
				assert.NotEmpty(testingT, unstruct)
				assert.NotNil(testingT, unstruct.GetAnnotations())
				assert.Contains(testingT, unstruct.GetAnnotations(), DisclaimerAnnotation)
				assert.Equal(testingT, disclaimerAnnotationValue, unstruct.GetAnnotations()[DisclaimerAnnotation])
				return true
			},
		},
		{
			"simple kymaComponentTransform",
			kymaComponentTransform,
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
			"simple managedByDeclarativeV2",
			managedByDeclarativeV2,
			[]*unstructured.Unstructured{{Object: map[string]any{}}},
			func(testingT assert.TestingT, err error, i ...interface{}) bool {
				require.NoError(t, err)
				unstructs, ok := i[0].([]*unstructured.Unstructured)
				assert.True(testingT, ok)
				unstruct := unstructs[0]
				assert.NotEmpty(testingT, unstruct)
				assert.NotNil(testingT, unstruct.GetLabels())
				assert.Contains(testingT, unstruct.GetLabels(), ManagedByLabel)
				assert.Equal(testingT, managedByLabelValue, unstruct.GetLabels()[ManagedByLabel])
				return true
			},
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(
			testCase.name, func(t *testing.T) {
				t.Parallel()
				obj := &testObj{Unstructured: &unstructured.Unstructured{}}
				obj.SetName("test-object")
				err := testCase.ObjectTransform(context.Background(), obj, testCase.resources)
				testCase.wantErr(
					t, err, testCase.resources,
				)
			},
		)
	}
}
