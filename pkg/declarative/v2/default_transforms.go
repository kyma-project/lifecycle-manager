package v2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	OperatorName   = "module-manager"
	OperatorPrefix = "operator.kyma-project.io"
	Separator      = "/"

	ManagedByLabel      = OperatorPrefix + Separator + "managed-by"
	managedByLabelValue = "declarative-v2"

	DisclaimerAnnotation      = OperatorPrefix + Separator + "managed-by-reconciler-disclaimer"
	disclaimerAnnotationValue = "DO NOT EDIT - This resource is managed by Kyma.\n" +
		"Any modifications are discarded and the resource is reverted to the original state."

	WatchedByLabel = OperatorPrefix + Separator + "watched-by"

	// OwnedByAnnotation defines the resource managing the resource. Differing from ManagedBy
	// in that it does not reference controllers. Used by the runtime-watcher to determine the
	// corresponding CR in KCP.
	OwnedByAnnotation = OperatorPrefix + Separator + "owned-by"
	OwnedByFormat     = "%s/%s"
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
		lbls[WatchedByLabel] = OperatorName

		annotations := resource.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[OwnedByAnnotation] = fmt.Sprintf(OwnedByFormat, obj.GetNamespace(), obj.GetName())
		resource.SetLabels(lbls)
	}
	return nil
}
