//nolint:testpackage // test private functions
package remote

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TestPrepareForSSA(t *testing.T) {
	t.Run("ensure required fields are modified", func(t *testing.T) {
		// given
		testModule := v1beta2.ModuleTemplate{}

		testModule.SetResourceVersion("1")
		testModule.SetUID("12")
		testModule.SetManagedFields([]apimetav1.ManagedFieldsEntry{stubManagedFieldsEntry()})
		testModule.SetLabels(map[string]string{"foo": "bar"})
		testModule.SetNamespace("default")

		assert.Equal(t, "1", testModule.GetResourceVersion())
		assert.EqualValues(t, "12", testModule.GetUID())
		assert.Len(t, testModule.GetManagedFields(), 1)
		assert.Equal(t, stubManagedFieldsEntry(), testModule.GetManagedFields()[0])
		assert.Len(t, testModule.GetLabels(), 1)
		assert.Equal(t, "bar", testModule.GetLabels()["foo"])
		assert.Equal(t, "default", testModule.GetNamespace())

		applyConfiguration := prepareModuleTemplateForSSA(&testModule, "someNamespace")

		assert.Nil(t, applyConfiguration.ResourceVersion)
		assert.Nil(t, applyConfiguration.UID)
		assert.Len(t, applyConfiguration.Labels, 2)
		assert.Equal(t, "bar", applyConfiguration.Labels["foo"])
		assert.Equal(t, shared.ManagedByLabelValue, applyConfiguration.Labels[shared.ManagedBy])
		assert.Equal(t, "someNamespace", *applyConfiguration.Namespace)
	})

	t.Run("ensure no fields of the original object are modified except the Namespace", func(t *testing.T) {
		// given
		expected := v1beta2.ModuleTemplate{}
		expected.SetNamespace("someNamespace")
		expectedJSON, err := json.Marshal(expected)
		require.NoError(t, err)

		testModule := v1beta2.ModuleTemplate{}

		// when
		prepareModuleTemplateForSSA(&testModule, "someNamespace")

		// then
		afterPrepareJSON, err := json.Marshal(testModule)
		require.NoError(t, err)
		assert.JSONEq(t, string(expectedJSON), string(afterPrepareJSON))
	})
}

func stubManagedFieldsEntry() apimetav1.ManagedFieldsEntry {
	return apimetav1.ManagedFieldsEntry{Manager: "1", Operation: apimetav1.ManagedFieldsOperationApply}
}
