package v2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

const (
	DisclaimerAnnotation      = shared.OperatorGroup + shared.Separator + "managed-by-reconciler-disclaimer"
	DisclaimerAnnotationValue = "DO NOT EDIT - This resource is managed by Kyma.\n" +
		"Any modifications are discarded and the resource is reverted to the original state."
	OwnedByFormat = "%s/%s"
)

func DisclaimerTransform(_ context.Context, _ Object, resources []*unstructured.Unstructured) error {
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

func KymaComponentTransform(_ context.Context, obj Object, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		resource.SetLabels(collections.MergeMapsSilent(resource.GetLabels(), map[string]string{
			"app.kubernetes.io/component": obj.GetName(),
			"app.kubernetes.io/part-of":   "Kyma",
		}))
	}
	return nil
}

func ManagedByOwnedBy(_ context.Context, obj Object, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		resource.SetLabels(collections.MergeMapsSilent(resource.GetLabels(), map[string]string{
			shared.ManagedBy: shared.ManagedByLabelValue,
		}))

		resource.SetAnnotations(collections.MergeMapsSilent(resource.GetAnnotations(), map[string]string{
			shared.OwnedByAnnotation: fmt.Sprintf(OwnedByFormat, obj.GetNamespace(), obj.GetName()),
		}))
	}
	return nil
}
