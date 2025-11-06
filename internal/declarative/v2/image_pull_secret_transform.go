package v2

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

//nolint:gosec // This is not sensitive information.
const SkrImagePullSecretEnvName = "SKR_IMG_PULL_SECRET"

var ErrResourceTransformExpectedManifestType = errors.New("resource transform expected Manifest type")

var ErrSkrImagePullSecretEnvAlreadyExists = errors.New(SkrImagePullSecretEnvName +
	" environment variable already exits in raw manifest")

func CreateSkrImagePullSecretTransform(secretName string) ResourceTransform {
	return func(_ context.Context, obj Object, resources []*unstructured.Unstructured) error {
		manifest, ok := obj.(*v1beta2.Manifest)
		if !ok {
			return fmt.Errorf("%w, got %T", ErrResourceTransformExpectedManifestType, obj)
		}
		for _, resource := range resources {
			if !isWorkloadResource(resource.GetKind()) {
				continue
			}
			podSpec, err := getWorkloadPodSpec(resource)
			if err != nil {
				return fmt.Errorf("failed to get pod spec for resource %s: %w", resource.GetName(), err)
			}
			patchImagePullSecrets(podSpec, secretName)
			if resourceIsManager(manifest, resource) {
				if err := patchEnvInContainers(podSpec, secretName); err != nil {
					return fmt.Errorf("failed to patch container envs in resource %s: %w", resource.GetName(), err)
				}
			}
			if err := setWorkloadPodSpec(resource, podSpec); err != nil {
				return fmt.Errorf("failed to set pod spec for resource %s: %w", resource.GetName(), err)
			}
		}
		return nil
	}
}

func isWorkloadResource(kind string) bool {
	return kind == "Deployment" || kind == "StatefulSet"
}

func getWorkloadPodSpec(resource *unstructured.Unstructured) (map[string]interface{}, error) {
	podSpec, _, err := unstructured.NestedMap(resource.Object, "spec", "template", "spec")
	return podSpec, err
}

func setWorkloadPodSpec(resource *unstructured.Unstructured, podSpec map[string]interface{}) error {
	return unstructured.SetNestedMap(resource.Object, podSpec, "spec", "template", "spec")
}

func resourceIsManager(manifest *v1beta2.Manifest, resource *unstructured.Unstructured) bool {
	manager := manifest.Spec.Manager
	if manager == nil {
		return true
	}
	return manager.Name == resource.GetName() &&
		manager.Namespace == resource.GetNamespace() &&
		manager.Group == resource.GroupVersionKind().Group &&
		manager.Version == resource.GroupVersionKind().Version &&
		manager.Kind == resource.GroupVersionKind().Kind
}

func patchImagePullSecrets(podSpec map[string]interface{}, secretName string) {
	imagePullSecrets, _ := podSpec["imagePullSecrets"].([]interface{})
	imagePullSecrets = append(imagePullSecrets, map[string]interface{}{"name": secretName})
	podSpec["imagePullSecrets"] = imagePullSecrets
}

func patchEnvInContainers(podSpec map[string]interface{}, secretName string) error {
	containers, _ := podSpec["containers"].([]interface{})
	if len(containers) == 0 {
		return nil
	}
	for index, container := range containers {
		containerMap, ok := container.(map[string]interface{})
		if !ok {
			continue
		}
		envSlice, _ := containerMap["env"].([]interface{})
		if envNameExists(envSlice, SkrImagePullSecretEnvName) {
			return ErrSkrImagePullSecretEnvAlreadyExists
		}
		envSlice = append(envSlice, map[string]interface{}{"name": SkrImagePullSecretEnvName, "value": secretName})
		containerMap["env"] = envSlice
		containers[index] = containerMap
	}
	podSpec["containers"] = containers
	return nil
}

func envNameExists(envSlice []interface{}, name string) bool {
	for _, env := range envSlice {
		envMap, ok := env.(map[string]interface{})
		if !ok {
			continue
		}
		if envMap["name"] == name {
			return true
		}
	}
	return false
}
