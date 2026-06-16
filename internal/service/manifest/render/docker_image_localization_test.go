package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
)

func TestDockerImageLocalizationTransform_NoImagesIsNoOp(t *testing.T) {
	t.Parallel()

	manifest := &v1beta2.Manifest{}
	resources := []*unstructured.Unstructured{deploymentWithImage(t, "controller", "registry.example/old:1.0.0")}

	require.NoError(t, render.DockerImageLocalizationTransform(t.Context(), manifest, resources))

	containers, found, err := unstructured.NestedSlice(resources[0].Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "registry.example/old:1.0.0", containers[0].(map[string]any)["image"])
}

func TestDockerImageLocalizationTransform_RewritesImagesOnMatch(t *testing.T) {
	t.Parallel()

	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{
			LocalizedImages: []string{"private-registry.com/prod/template-operator:1.0.3"},
		},
	}
	resources := []*unstructured.Unstructured{
		deploymentWithImage(t, "manager", "europe-docker.pkg.dev/kyma-project/prod/template-operator:1.0.3"),
	}

	require.NoError(t, render.DockerImageLocalizationTransform(t.Context(), manifest, resources))

	containers, found, err := unstructured.NestedSlice(resources[0].Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "private-registry.com/prod/template-operator:1.0.3", containers[0].(map[string]any)["image"])
}

func TestDockerImageLocalizationTransform_InvalidLocalizedImageReturnsError(t *testing.T) {
	t.Parallel()

	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{LocalizedImages: []string{"::not a valid ref::"}},
	}

	err := render.DockerImageLocalizationTransform(t.Context(), manifest, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse localized images")
}

func deploymentWithImage(t *testing.T, container, image string) *unstructured.Unstructured {
	t.Helper()
	d := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "manager", "namespace": "default"},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": container, "image": image},
						},
					},
				},
			},
		},
	}
	return d
}
