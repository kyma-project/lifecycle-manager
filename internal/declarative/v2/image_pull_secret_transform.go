package v2

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const SkrImagePullSecretEnvName = "SKR_IMG_PULL_SECRET"

var ErrSkrImagePullSecretEnvAlreadyExists = errors.New(SkrImagePullSecretEnvName + " environment variable already exists")

func CreateSkrImagePullSecretTransform(secretName string) ResourceTransform {
	return func(_ context.Context, _ Object, resources []*unstructured.Unstructured) error {
		for _, resource := range resources {
			err := patchDeploymentsAndStatefulSets(resource, secretName)
			if err != nil {
				return fmt.Errorf("failed to patch SKR image pull secret in resource %s: %w", resource.GetName(), err)
			}
		}
		return nil
	}
}

func patchDeploymentsAndStatefulSets(resource *unstructured.Unstructured, secretName string) error {
	if resource.GetKind() == "Deployment" || resource.GetKind() == "StatefulSet" {
		err := parseAndPatchAllContainers(resource, secretName)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseAndPatchAllContainers(resource *unstructured.Unstructured, secretName string) error {
	containers, found, err := unstructured.NestedSlice(resource.Object, "spec", "template", "spec",
		"containers")
	if err != nil {
		return fmt.Errorf("failed to get containers: %w", err)
	}
	if found {
		err = patchContainers(containers, secretName)
		if err != nil {
			return err
		}
		if err := unstructured.SetNestedSlice(resource.Object, containers, "spec", "template", "spec",
			"containers"); err != nil {
			return fmt.Errorf("failed to set containers: %w", err)
		}
	}
	return nil
}

func patchContainers(containers []interface{}, secretName string) error {
	for index, container := range containers {
		containerMap := container.(map[string]interface{})
		envSlice := getEnvSliceFromContainerMap(containerMap)
		if isReservedEnvNameOccupied(envSlice) {
			return ErrSkrImagePullSecretEnvAlreadyExists
		}
		addImagePullSecretToEnvSlice(containerMap, envSlice, secretName)
		containers[index] = containerMap
	}
	return nil
}

func getEnvSliceFromContainerMap(containerMap map[string]interface{}) []interface{} {
	envSlice, ok := containerMap["env"].([]interface{})
	if !ok {
		envSlice = []interface{}{}
	}
	return envSlice
}

func isReservedEnvNameOccupied(envSlice []interface{}) bool {
	for _, env := range envSlice {
		envMap, ok := env.(map[string]interface{})
		if !ok {
			continue
		}
		if envMap["name"] == SkrImagePullSecretEnvName {
			return true
		}
	}
	return false
}

func addImagePullSecretToEnvSlice(containerMap map[string]interface{}, envSlice []interface{}, secretName string) {
	containerMap["env"] = append(envSlice, map[string]interface{}{
		"name":  SkrImagePullSecretEnvName,
		"value": secretName,
	})
}
