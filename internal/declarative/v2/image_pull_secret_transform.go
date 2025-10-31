package v2

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//nolint:gosec // This is not sensitive information.
const SkrImagePullSecretEnvName = "SKR_IMG_PULL_SECRET"

var ErrSkrImagePullSecretEnvAlreadyExists = errors.New(SkrImagePullSecretEnvName +
	" environment variable already exits in raw manifest")

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
		err := parseAndPatchImagePullSecrets(resource, secretName)
		if err != nil {
			return fmt.Errorf("failed to parse and patch imagePullSecrets in pod spec: %w", err)
		}
		err = parseAndPatchAllContainerEnvs(resource, secretName)
		if err != nil {
			return fmt.Errorf("failed to parse and patch container envs: %w", err)
		}
	}
	return nil
}

func parseAndPatchImagePullSecrets(resource *unstructured.Unstructured, secretName string) error {
	podSpec, found, err := unstructured.NestedMap(resource.Object, "spec", "template", "spec")
	if err != nil {
		return fmt.Errorf("failed to get pod spec: %w", err)
	}
	if found {
		err = patchPodSpec(podSpec, secretName)
		if err != nil {
			return err
		}
		if err := unstructured.SetNestedMap(resource.Object, podSpec, "spec", "template", "spec"); err != nil {
			return fmt.Errorf("failed to set pod spec: %w", err)
		}
	}
	return nil
}

func patchPodSpec(podSpec map[string]interface{}, secretName string) error {
	imagePullSecrets := getImagePullSecretsFromPodSpec(podSpec)
	imagePullSecrets = appendSecretNameToImagePullSecrets(imagePullSecrets, secretName)
	setImagePullSecretsToPodSpec(podSpec, imagePullSecrets)
	return nil
}

func setImagePullSecretsToPodSpec(podSpec map[string]interface{}, imagePullSecrets []interface{}) {
	podSpec["imagePullSecrets"] = imagePullSecrets
}

func getImagePullSecretsFromPodSpec(podSpec map[string]interface{}) []interface{} {
	imagePullSecrets, ok := podSpec["imagePullSecrets"].([]interface{})
	if !ok {
		imagePullSecrets = []interface{}{}
	}
	return imagePullSecrets
}

func appendSecretNameToImagePullSecrets(imagePullSecrets []interface{}, secretName string) []interface{} {
	return append(imagePullSecrets, map[string]interface{}{
		"name": secretName,
	})
}

func parseAndPatchAllContainerEnvs(resource *unstructured.Unstructured, secretName string) error {
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
		containerMap, ok := container.(map[string]interface{})
		if !ok {
			continue
		}
		env := getEnvFromContainerMap(containerMap)
		if isReservedEnvNameOccupied(env) {
			return ErrSkrImagePullSecretEnvAlreadyExists
		}
		env = appendSecretNameToEnvSlice(env, secretName)
		setEnvToContainerMap(containerMap, env)
		containers[index] = containerMap
	}
	return nil
}

func getEnvFromContainerMap(containerMap map[string]interface{}) []interface{} {
	env, ok := containerMap["env"].([]interface{})
	if !ok {
		env = []interface{}{}
	}
	return env
}

func isReservedEnvNameOccupied(env []interface{}) bool {
	for _, e := range env {
		envMap, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if envMap["name"] == SkrImagePullSecretEnvName {
			return true
		}
	}
	return false
}

func appendSecretNameToEnvSlice(env []interface{}, secretName string) []interface{} {
	return append(env, map[string]interface{}{
		"name":  SkrImagePullSecretEnvName,
		"value": secretName,
	})
}

func setEnvToContainerMap(containerMap map[string]interface{}, env []interface{}) {
	containerMap["env"] = env
}
