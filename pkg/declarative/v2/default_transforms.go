package v2

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/pkg/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DisclaimerAnnotation      = "reconciler.kyma-project.io/managed-by-reconciler-disclaimer"
	disclaimerAnnotationValue = "DO NOT EDIT - This resource is managed by Kyma.\n" +
		"Any modifications are discarded and the resource is reverted to the original state."
	ManagedByLabel      = "reconciler.kyma-project.io/managed-by"
	managedByLabelValue = "declarative-v2"
)

func disclaimerTransform(_ context.Context, _ Object, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		annotations := resource.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[DisclaimerAnnotation] = disclaimerAnnotationValue
		resource.SetAnnotations(annotations)
	}
	return nil
}

func kymaComponentTransform(_ context.Context, obj Object, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		lbls := resource.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		lbls["app.kubernetes.io/component"] = obj.ComponentName()
		lbls["app.kubernetes.io/part-of"] = "Kyma"
		resource.SetLabels(lbls)
	}
	return nil
}

func managedByDeclarativeV2(_ context.Context, _ Object, resources []*unstructured.Unstructured) error {
	for _, resource := range resources {
		lbls := resource.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		// legacy managed by value
		lbls[ManagedByLabel] = managedByLabelValue
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
		lbls[labels.WatchedByLabel] = labels.OperatorName
		lbls[labels.OwnedByLabel] = obj.GetName()
		resource.SetLabels(lbls)
	}
	return nil
}
