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
			Object: map[string]any{
				"kind": "Service",
				"spec": map[string]any{},
			},
		},
		{
			Object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{},
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

			secretRef := imagePullSecrets[0].(map[string]any)
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
			Object: map[string]any{
				"kind": "Service",
				"spec": map[string]any{},
			},
		},
		{
			Object: map[string]any{
				"kind": "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"imagePullSecrets": []any{
								map[string]any{
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

			existingSecret := imagePullSecrets[0].(map[string]any)
			require.Equal(t, "existing-secret", existingSecret["name"])
			secretRef := imagePullSecrets[1].(map[string]any)
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
			Object: map[string]any{
				"kind": "Service",
				"spec": map[string]any{},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "manager-deployment",
					"namespace": "default",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "manager",
									"image": "controller:latest",
								},
								map[string]any{
									"name":  "sidecar",
									"image": "sidecar:latest",
									"env": []any{
										map[string]any{
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
				containerMap := c.(map[string]any)
				envSlice, found, err := unstructured.NestedSlice(containerMap, "env")
				require.NoError(t, err)
				require.True(t, found)

				var skrEnvFound bool
				for _, envVar := range envSlice {
					envVarMap := envVar.(map[string]any)
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

func TestCreateSkrImagePullSecretTransform_WhenEnvDoesntExist_AndManagerNotSpecified_AddsEnv(t *testing.T) {
	t.Parallel()
	secretName := random.Name()

	transform := declarativev2.CreateSkrImagePullSecretTransform(secretName)
	manifest := &v1beta2.Manifest{
		Spec: v1beta2.ManifestSpec{},
	}
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "some-deployment",
					"namespace": "default",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "some-container",
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

	for _, resource := range resources {
		if resource.GetKind() == "Deployment" {
			containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec",
				"containers")
			require.NoError(t, err)
			require.True(t, found)
			require.Len(t, containers, 1)

			containerMap := containers[0].(map[string]any)
			envSlice, found, err := unstructured.NestedSlice(containerMap, "env")
			require.NoError(t, err)
			require.True(t, found)

			var skrEnvFound bool
			for _, envVar := range envSlice {
				envVarMap := envVar.(map[string]any)
				if envVarMap["name"] == declarativev2.SkrImagePullSecretEnvName {
					skrEnvFound = true
					require.Equal(t, secretName, envVarMap["value"])
				}
			}
			require.True(t, skrEnvFound)
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
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "manager-deployment",
					"namespace": "default",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "manager",
									"image": "controller:latest",
									"env": []any{
										map[string]any{
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
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "some-other-deployment",
					"namespace": "default",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
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

	containerMap := containers[0].(map[string]any)
	_, found, err = unstructured.NestedSlice(containerMap, "env")
	require.NoError(t, err)
	require.False(t, found)
}
