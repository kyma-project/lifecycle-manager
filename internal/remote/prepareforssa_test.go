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

		prepareModuleTemplateForSSA(&testModule, "someNamespace")

		assert.Empty(t, testModule.GetResourceVersion())
		assert.Empty(t, testModule.GetUID())
		assert.Empty(t, testModule.GetManagedFields())
		assert.Len(t, testModule.GetLabels(), 2)
		assert.Equal(t, "bar", testModule.GetLabels()["foo"])
		assert.Equal(t, shared.ManagedByLabelValue, testModule.GetLabels()[shared.ManagedBy])
		assert.Equal(t, "someNamespace", testModule.GetNamespace())
	})

	t.Run("ensure no other fields are modified", func(t *testing.T) {
		// given
		testModule := v1beta2.ModuleTemplate{}
		prepareModuleTemplateForSSA(&testModule, "someNamespace")

		afterPrepareJSON, err := json.Marshal(testModule)
		require.NoError(t, err)

		expected := v1beta2.ModuleTemplate{}
		expected.SetNamespace("someNamespace")
		expected.SetLabels(map[string]string{shared.ManagedBy: shared.ManagedByLabelValue})
		expectedJSON, err := json.Marshal(expected)
		require.NoError(t, err)

		assert.JSONEq(t, string(expectedJSON), string(afterPrepareJSON))
	})
}

func stubManagedFieldsEntry() apimetav1.ManagedFieldsEntry {
	return apimetav1.ManagedFieldsEntry{Manager: "1", Operation: apimetav1.ManagedFieldsOperationApply}
}
