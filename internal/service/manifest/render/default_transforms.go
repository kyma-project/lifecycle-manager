package render

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/imagerewrite"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

const (
	DisclaimerAnnotation      = shared.OperatorGroup + shared.Separator + "managed-by-reconciler-disclaimer"
	DisclaimerAnnotationValue = "DO NOT EDIT - This resource is managed by Kyma.\n" +
		"Any modifications are discarded and the resource is reverted to the original state."
)

func DisclaimerTransform(_ context.Context, _ *v1beta2.Manifest, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		annotations := resource.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[DisclaimerAnnotation] = DisclaimerAnnotationValue
		resource.SetAnnotations(annotations)
	}
	return nil
}

// DockerImageLocalizationTransform rewrites Docker images in the provided resources
// according to the Spec.LocalizedImages field in the Manifest object.
func DockerImageLocalizationTransform(_ context.Context, manifest *v1beta2.Manifest,
	resources []*unstructured.Unstructured,
) error {
	if len(manifest.Spec.LocalizedImages) == 0 {
		return nil // No images to rewrite
	}

	localizedImages, err := imagerewrite.AsImageReferences(manifest.Spec.LocalizedImages)
	if err != nil {
		return fmt.Errorf("failed to parse localized images: %w", err)
	}

	rewriter := (&imagerewrite.ResourceRewriter{}).WithRewriters(
		&imagerewrite.PodContainerImageRewriter{},
		&imagerewrite.PodContainerEnvsRewriter{},
	)

	for _, resource := range resources {
		if err = rewriter.ReplaceImages(resource, localizedImages); err != nil {
			return fmt.Errorf(
				"failed to rewrite images in resource %s/%s: %w",
				resource.GetNamespace(),
				resource.GetName(),
				err,
			)
		}
	}
	return nil
}

func KymaComponentTransform(_ context.Context, manifest *v1beta2.Manifest,
	resources []*unstructured.Unstructured,
) error {
	for _, resource := range resources {
		resource.SetLabels(collections.MergeMapsSilent(resource.GetLabels(), map[string]string{
			"app.kubernetes.io/component": manifest.GetName(),
			"app.kubernetes.io/part-of":   "Kyma",
		}))
	}
	return nil
}

func ManagedByOwnedBy(_ context.Context, manifest *v1beta2.Manifest,
	resources []*unstructured.Unstructured,
) error {
	for _, resource := range resources {
		resource.SetLabels(collections.MergeMapsSilent(resource.GetLabels(), map[string]string{
			shared.ManagedBy: shared.ManagedByLabelValue,
		}))

		resource.SetAnnotations(collections.MergeMapsSilent(resource.GetAnnotations(), map[string]string{
			shared.OwnedByAnnotation: fmt.Sprintf(shared.OwnedByFormat, manifest.GetNamespace(), manifest.GetName()),
		}))
	}
	return nil
}

func GetDefaultResourceTransforms() []ResourceTransform {
	return []ResourceTransform{
		ManagedByOwnedBy,
		KymaComponentTransform,
		DisclaimerTransform,
		DockerImageLocalizationTransform,
	}
}
