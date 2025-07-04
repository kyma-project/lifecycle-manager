package imagerewrite

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ErrInvalidContainerType = errors.New("invalid container type, expected map[string]any")

// ImageRewriter is an object that rewrites images in-place in some specific section of a Kubernetes Pod container.
type ImageRewriter interface {
	Rewrite(targetImages []TargetImage, podContainer *unstructured.Unstructured) error
}

// ResourceRewriter rewrites the host and path of the images in the kubernetes resources based on the localized images specified in the manifest.
// The target resource must represent a Kubernetes object that contains a Pod template, such as a Deployment or StatefulSet.
// The rewriter modifies images specified in the following places:
//   - all pod template container images
//   - all pod template environment variables that contain image references
type ResourceRewriter struct {
	rewriters []ImageRewriter
}

func (r *ResourceRewriter) WithRewriters(rewriters ...ImageRewriter) *ResourceRewriter {
	r.rewriters = append(r.rewriters, rewriters...)
	return r
}

// ReplaceImages replaces images in the given Kubernetes resource with the target images.
// It expects the resource to have a Pod template (e.g., Deployment, StatefulSet).
func (r *ResourceRewriter) ReplaceImages(deploymentOrSimilar *unstructured.Unstructured, targetImages []TargetImage) error {
	podContainersGetter := func() ([]*unstructured.Unstructured, error) {
		containers, err := getPodContainers(deploymentOrSimilar)
		if err != nil {
			return nil, fmt.Errorf("failed to get pod containers: %w", err)
		}
		return containers, nil
	}
	podContainersSetter := func(containers []*unstructured.Unstructured) error {
		if err := setPodContainers(deploymentOrSimilar, containers); err != nil {
			return fmt.Errorf("failed to set pod containers: %w", err)
		}
		return nil
	}

	err := r.rewriteContainers(podContainersGetter, podContainersSetter, targetImages)
	if err != nil {
		return fmt.Errorf("failed to rewrite pod containers: %w", err)
	}

	return nil
}

type (
	podContainersGetterFn func() ([]*unstructured.Unstructured, error)
	podContainersSetterFn func([]*unstructured.Unstructured) error
)

func (r *ResourceRewriter) rewriteContainers(containersGetter podContainersGetterFn, containersSetter podContainersSetterFn, targetImages []TargetImage) error {
	containers, err := containersGetter()
	if err != nil {
		return err
	}
	for cIdx, container := range containers {
		for _, rewriter := range r.rewriters {
			if err := rewriter.Rewrite(targetImages, container); err != nil {
				return fmt.Errorf("failed to rewrite images in container %d: %w", cIdx, err)
			}
		}
	}

	if err := containersSetter(containers); err != nil {
		return err
	}
	return nil
}

// getPodContainers retrieves the containers from a Kubernetes resource that has a Pod template.
// It returns a slice of unstructured.Unstructured representing the Pod containers.
// deploymentOrSimilar is expected to be a Deployment, StatefulSet, or similar resource that contains a Pod template.
func getPodContainers(deploymentOrSimilar *unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	containers, found, err := unstructured.NestedSlice(deploymentOrSimilar.Object, "spec", "template", "spec", "containers")
	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}

	if !found || len(containers) == 0 {
		return nil, nil // No containers found
	}

	containerResources := make([]*unstructured.Unstructured, len(containers))
	for i, container := range containers {
		containerMap, ok := container.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%T: %w", container, ErrInvalidContainerType)
		}
		containerResources[i] = &unstructured.Unstructured{Object: containerMap}
	}
	return containerResources, nil
}

func setPodContainers(deploymentOrSimilar *unstructured.Unstructured, containers []*unstructured.Unstructured) error {
	containerObjects := make([]any, len(containers))
	for i, container := range containers {
		containerObjects[i] = container.Object
	}

	err := unstructured.SetNestedSlice(deploymentOrSimilar.Object, containerObjects, "spec", "template", "spec", "containers")
	if err != nil {
		return fmt.Errorf("failed to set pod containers: %w", err)
	}
	return nil
}
