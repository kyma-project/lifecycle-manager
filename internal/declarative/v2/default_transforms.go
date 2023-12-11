package v2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	OperatorName              = "module-manager"
	ManagedByLabelValue       = "declarative-v2"
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
		lbls := resource.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		lbls["app.kubernetes.io/component"] = obj.GetName()
		lbls["app.kubernetes.io/part-of"] = "Kyma"
		resource.SetLabels(lbls)
	}
	return nil
}

func ManagedByDeclarativeV2(_ context.Context, _ Object, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		lbls := resource.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		// legacy managed by value
		lbls[shared.ManagedBy] = ManagedByLabelValue
		resource.SetLabels(lbls)
	}
	return nil
}

func watchedByOwnedBy(_ context.Context, obj Object, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		lbls := resource.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		// legacy managed by value
		lbls[shared.WatchedByLabel] = OperatorName

		annotations := resource.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[shared.OwnedByAnnotation] = fmt.Sprintf(OwnedByFormat, obj.GetNamespace(), obj.GetName())
		resource.SetLabels(lbls)
	}
	return nil
}
