package v2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

func TestNormaliseNamespaceTransform_SetsDefaultNamespaceOnNamespacedResource(t *testing.T) {
	t.Parallel()
	skrClient := newFakeSkrClient(map[schema.GroupVersionKind]meta.RESTScope{
		{Group: "", Version: "v1", Kind: "ConfigMap"}: meta.RESTScopeNamespace,
	})

	configMap := newUnstructured("v1", "ConfigMap", "cm", "")

	err := declarativev2.NormaliseNamespaceTransform(t.Context(), skrClient, nil,
		[]*unstructured.Unstructured{configMap})

	require.NoError(t, err)
	assert.Equal(t, "default", configMap.GetNamespace())
}

func TestNormaliseNamespaceTransform_PreservesExistingNamespaceOnNamespacedResource(t *testing.T) {
	t.Parallel()
	skrClient := newFakeSkrClient(map[schema.GroupVersionKind]meta.RESTScope{
		{Group: "", Version: "v1", Kind: "ConfigMap"}: meta.RESTScopeNamespace,
	})

	configMap := newUnstructured("v1", "ConfigMap", "cm", "kyma-system")

	err := declarativev2.NormaliseNamespaceTransform(t.Context(), skrClient, nil,
		[]*unstructured.Unstructured{configMap})

	require.NoError(t, err)
	assert.Equal(t, "kyma-system", configMap.GetNamespace())
}

func TestNormaliseNamespaceTransform_StripsNamespaceFromClusterScopedResource(t *testing.T) {
	t.Parallel()
	skrClient := newFakeSkrClient(map[schema.GroupVersionKind]meta.RESTScope{
		{Group: "", Version: "v1", Kind: "Namespace"}: meta.RESTScopeRoot,
	})

	namespace := newUnstructured("v1", "Namespace", "kyma-system", "kyma-system")

	err := declarativev2.NormaliseNamespaceTransform(t.Context(), skrClient, nil,
		[]*unstructured.Unstructured{namespace})

	require.NoError(t, err)
	assert.Empty(t, namespace.GetNamespace())
}

func TestNormaliseNamespaceTransform_TolerateUnknownGVK(t *testing.T) {
	t.Parallel()
	skrClient := newFakeSkrClient(nil)

	unknown := newUnstructured("custom.io/v1", "Widget", "w", "")

	err := declarativev2.NormaliseNamespaceTransform(t.Context(), skrClient, nil,
		[]*unstructured.Unstructured{unknown})

	require.NoError(t, err, "unknown GVK must be passed through (NoMatchError is recoverable)")
}

func newUnstructured(apiVersion, kind, name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	return obj
}

// newFakeSkrClient returns a skrclient.Client backed by controller-runtime's
// fake client, with a RESTMapper configured for the given GVK→scope entries.
func newFakeSkrClient(scopes map[schema.GroupVersionKind]meta.RESTScope) skrclient.Client {
	groupVersions := make([]schema.GroupVersion, 0, len(scopes))
	for gvk := range scopes {
		groupVersions = append(groupVersions, gvk.GroupVersion())
	}
	mapper := meta.NewDefaultRESTMapper(groupVersions)
	for gvk, scope := range scopes {
		mapper.Add(gvk, scope)
	}
	return fake.NewClientBuilder().WithRESTMapper(mapper).Build()
}
