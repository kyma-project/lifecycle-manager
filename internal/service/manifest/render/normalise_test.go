package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

var (
	configMapGVK = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	namespaceGVK = schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}

	namespacedConfigMap = map[schema.GroupVersionKind]meta.RESTScope{
		configMapGVK: meta.RESTScopeNamespace,
	}
	clusterScopedNamespace = map[schema.GroupVersionKind]meta.RESTScope{
		namespaceGVK: meta.RESTScopeRoot,
	}
)

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

func TestNormaliseNamespace_DefaultsNamespaceOnNamespacedResource(t *testing.T) {
	t.Parallel()

	configMap := &unstructured.Unstructured{}
	configMap.SetGroupVersionKind(configMapGVK)
	configMap.SetName("cm")

	err := render.NormaliseNamespace(configMap, "default", newFakeSkrClient(namespacedConfigMap))

	require.NoError(t, err)
	assert.Equal(t, "default", configMap.GetNamespace())
}

func TestNormaliseNamespace_PreservesExistingNamespace(t *testing.T) {
	t.Parallel()

	configMap := &unstructured.Unstructured{}
	configMap.SetGroupVersionKind(configMapGVK)
	configMap.SetName("cm")
	configMap.SetNamespace("kyma-system")

	err := render.NormaliseNamespace(configMap, "default", newFakeSkrClient(namespacedConfigMap))

	require.NoError(t, err)
	assert.Equal(t, "kyma-system", configMap.GetNamespace())
}

func TestNormaliseNamespace_StripsNamespaceFromClusterScopedResource(t *testing.T) {
	t.Parallel()

	namespace := &unstructured.Unstructured{}
	namespace.SetGroupVersionKind(namespaceGVK)
	namespace.SetName("kyma-system")
	namespace.SetNamespace("kyma-system")

	err := render.NormaliseNamespace(namespace, "default", newFakeSkrClient(clusterScopedNamespace))

	require.NoError(t, err)
	assert.Empty(t, namespace.GetNamespace())
}

func TestNormaliseNamespace_ReturnsNoMatchErrorForUnknownGVK(t *testing.T) {
	t.Parallel()

	widget := &unstructured.Unstructured{}
	widget.SetGroupVersionKind(schema.GroupVersionKind{Group: "custom.io", Version: "v1", Kind: "Widget"})
	widget.SetName("w")

	err := render.NormaliseNamespace(widget, "default", newFakeSkrClient(nil))

	require.Error(t, err)
	assert.True(t, meta.IsNoMatchError(err),
		"unknown GVK must surface as a recoverable NoMatchError so the Service can skip it")
}
