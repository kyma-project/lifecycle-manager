package v2_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestImagePullSecretTransform_WhenNoImagePullSecretsExists_CreatesNew(t *testing.T) {
	t.Parallel()
	secretName := random.Name()

	transform := declarativev2.CreateSkrImagePullSecretTransform(secretName)
	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{},
	}
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"kind": "Service",
				"spec": map[string]interface{}{},
			},
		},
		{
			Object: map[string]interface{}{
				"kind": "Deployment",
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{},
					},
				},
			},
		},
	}

	err := transform(t.Context(), manifest, resources)
	require.NoError(t, err)

	for _, resource := range resources {
		if resource.GetKind() == "Deployment" {
			podSpec, found, err := unstructured.NestedMap(resource.Object, "spec", "template", "spec")
			require.NoError(t, err)
			require.True(t, found)

			imagePullSecrets, found, err := unstructured.NestedSlice(podSpec, "imagePullSecrets")
			require.NoError(t, err)
			require.True(t, found)
			require.Len(t, imagePullSecrets, 1)

			secretRef := imagePullSecrets[0].(map[string]interface{})
			require.Equal(t, secretName, secretRef["name"])
		}
	}
}

func TestImagePullSecretTransform_WhenImagePullSecretsExists_Appends(t *testing.T) {
	t.Parallel()
	secretName := random.Name()

	transform := declarativev2.CreateSkrImagePullSecretTransform(secretName)
	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{},
	}
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"kind": "Service",
				"spec": map[string]interface{}{},
			},
		},
		{
			Object: map[string]interface{}{
				"kind": "Deployment",
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"imagePullSecrets": []interface{}{
								map[string]interface{}{
									"name": "existing-secret",
								},
							},
						},
					},
				},
			},
		},
	}

	err := transform(t.Context(), manifest, resources)
	require.NoError(t, err)

	for _, resource := range resources {
		if resource.GetKind() == "Deployment" {
			podSpec, found, err := unstructured.NestedMap(resource.Object, "spec", "template", "spec")
			require.NoError(t, err)
			require.True(t, found)

			imagePullSecrets, found, err := unstructured.NestedSlice(podSpec, "imagePullSecrets")
			require.NoError(t, err)
			require.True(t, found)
			require.Len(t, imagePullSecrets, 2)

			existingSecret := imagePullSecrets[0].(map[string]interface{})
			require.Equal(t, "existing-secret", existingSecret["name"])
			secretRef := imagePullSecrets[1].(map[string]interface{})
			require.Equal(t, secretName, secretRef["name"])
		}
	}
}

func TestCreateSkrImagePullSecretTransform_WhenEnvDoesntExist_AddsEnv(t *testing.T) {
	t.Parallel()
	secretName := random.Name()

	transform := declarativev2.CreateSkrImagePullSecretTransform(secretName)
	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{
			Manager: &v1beta2.Manager{
				Name:      "manager-deployment",
				Namespace: "default",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Kind:    "Deployment",
					Version: "v1",
					Group:   "apps",
				},
			},
		},
	}
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"kind": "Service",
				"spec": map[string]interface{}{},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "manager-deployment",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "manager",
									"image": "controller:latest",
								},
								map[string]interface{}{
									"name":  "sidecar",
									"image": "sidecar:latest",
									"env": []interface{}{
										map[string]interface{}{
											"name":  "SOME_ENV",
											"value": "some_value",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	err := transform(t.Context(), manifest, resources)
	require.NoError(t, err)

	for _, resource := range resources {
		if resource.GetKind() == "Deployment" {
			containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec",
				"containers")
			require.NoError(t, err)
			require.True(t, found)
			require.Len(t, containers, 2)

			for _, c := range containers {
				containerMap := c.(map[string]interface{})
				envSlice, found, err := unstructured.NestedSlice(containerMap, "env")
				require.NoError(t, err)
				require.True(t, found)

				var skrEnvFound bool
				for _, envVar := range envSlice {
					envVarMap := envVar.(map[string]interface{})
					if envVarMap["name"] == declarativev2.SkrImagePullSecretEnvName {
						skrEnvFound = true
						require.Equal(t, secretName, envVarMap["value"])
					}
				}
				require.True(t, skrEnvFound)
			}
		}
	}
}

func TestCreateSkrImagePullSecretTransform_WhenEnvExists_ReturnsError(t *testing.T) {
	t.Parallel()
	secretName := random.Name()

	transform := declarativev2.CreateSkrImagePullSecretTransform(secretName)
	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{
			Manager: &v1beta2.Manager{
				Name:      "manager-deployment",
				Namespace: "default",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Kind:    "Deployment",
					Version: "v1",
					Group:   "apps",
				},
			},
		},
	}
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "manager-deployment",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "manager",
									"image": "controller:latest",
									"env": []interface{}{
										map[string]interface{}{
											"name":  declarativev2.SkrImagePullSecretEnvName,
											"value": "some_value",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	err := transform(t.Context(), manifest, resources)
	require.ErrorIs(t, err, declarativev2.ErrSkrImagePullSecretEnvAlreadyExists)
}

func TestCreateSkrImagePullSecretTransform_DoesntAddEnvVarToNonManagerDeployment(t *testing.T) {
	t.Parallel()
	secretName := random.Name()

	transform := declarativev2.CreateSkrImagePullSecretTransform(secretName)
	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{
			Manager: &v1beta2.Manager{
				Name:      "manager-deployment",
				Namespace: "default",
				GroupVersionKind: apimetav1.GroupVersionKind{
					Kind:    "Deployment",
					Version: "v1",
					Group:   "apps",
				},
			},
		},
	}
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "some-other-deployment",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "manager",
									"image": "controller:latest",
								},
							},
						},
					},
				},
			},
		},
	}

	err := transform(t.Context(), manifest, resources)
	require.NoError(t, err)

	containers, found, err := unstructured.NestedSlice(resources[0].Object, "spec", "template", "spec",
		"containers")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, containers, 1)

	containerMap := containers[0].(map[string]interface{})
	_, found, err = unstructured.NestedSlice(containerMap, "env")
	require.NoError(t, err)
	require.False(t, found)
}
