package imagerewrite

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ErrInvalidContainerType = errors.New("invalid container type, expected map[string]any")

// ImageRewriter is an object that rewrites images in-place in some specific section of a Kubernetes Pod container.
type ImageRewriter interface {
	Rewrite(targetImages []*DockerImageReference, podContainer *unstructured.Unstructured) error
}

// ResourceRewriter rewrites the host and path of the images in the kubernetes resources
// based on the localized images specified in the manifest.
// The target resource must represent a Kubernetes object that contains a Pod template,
// such as a Deployment or StatefulSet.
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
// It supports only Deployment and StatefulSet kinds, it ignores any unsupported resources.
func (r *ResourceRewriter) ReplaceImages(
	resource *unstructured.Unstructured,
	targetImages []*DockerImageReference,
) error {
	if !IsSupportedKind(resource.GetKind()) {
		return nil
	}

	deploymentOrSimilar := resource
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

	podInitContainersGetter := func() ([]*unstructured.Unstructured, error) {
		containers, err := getPodInitContainers(deploymentOrSimilar)
		if err != nil {
			return nil, fmt.Errorf("failed to get pod init containers: %w", err)
		}
		return containers, nil
	}
	podInitContainersSetter := func(containers []*unstructured.Unstructured) error {
		if err := setPodInitContainers(deploymentOrSimilar, containers); err != nil {
			return fmt.Errorf("failed to set pod init containers: %w", err)
		}
		return nil
	}
	err = r.rewriteContainers(podInitContainersGetter, podInitContainersSetter, targetImages)
	if err != nil {
		return fmt.Errorf("failed to rewrite pod init containers: %w", err)
	}

	return nil
}

type (
	podContainersGetterFn func() ([]*unstructured.Unstructured, error)
	podContainersSetterFn func([]*unstructured.Unstructured) error
)

func (r *ResourceRewriter) rewriteContainers(
	containersGetter podContainersGetterFn,
	containersSetter podContainersSetterFn,
	targetImages []*DockerImageReference,
) error {
	containers, err := containersGetter()
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return nil // No containers to rewrite, nothing to do.
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
	return getContainersGeneric(func() ([]any, bool, error) {
		return unstructured.NestedSlice(deploymentOrSimilar.Object, "spec", "template", "spec", "containers")
	})
}

func getPodInitContainers(deploymentOrSimilar *unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	return getContainersGeneric(func() ([]any, bool, error) {
		return unstructured.NestedSlice(deploymentOrSimilar.Object, "spec", "template", "spec", "initContainers")
	})
}

func setPodContainers(deploymentOrSimilar *unstructured.Unstructured, containers []*unstructured.Unstructured) error {
	return setContainersGeneric(containers, func(containerObjects []any) error {
		return unstructured.SetNestedSlice(
			deploymentOrSimilar.Object,
			containerObjects,
			"spec",
			"template",
			"spec",
			"containers",
		)
	})
}

func setPodInitContainers(
	deploymentOrSimilar *unstructured.Unstructured,
	containers []*unstructured.Unstructured,
) error {
	return setContainersGeneric(containers, func(containerObjects []any) error {
		return unstructured.SetNestedSlice(
			deploymentOrSimilar.Object,
			containerObjects,
			"spec",
			"template",
			"spec",
			"initContainers",
		)
	})
}

func getContainersGeneric(getNestedSliceFn func() ([]any, bool, error)) ([]*unstructured.Unstructured, error) {
	containers, found, err := getNestedSliceFn()
	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}

	if !found || len(containers) == 0 {
		return nil, nil // No containers found: It's normal case for initContainers.
		// For "standard" containers, it should not happen but we don't have to error out here -
		// it's the API Server job to prevent this.
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

func setContainersGeneric(containers []*unstructured.Unstructured, setNestedSliceFn func([]any) error) error {
	if len(containers) == 0 {
		return nil // No containers to set, nothing to do.
	}

	containerObjects := make([]any, len(containers))
	for i, container := range containers {
		containerObjects[i] = container.Object
	}

	return setNestedSliceFn(containerObjects)
}
