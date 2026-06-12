package v2

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

// NormaliseNamespaceTransform fixes malformed resources, e.g. produced by bad
// charts or wrong type configs, so that namespaced resources carry a namespace
// (defaulted to apimetav1.NamespaceDefault) and cluster-scoped resources do
// not. Resources whose GVK is unknown to the SKR's REST mapper are passed
// through unchanged.
func NormaliseNamespaceTransform(_ context.Context, skrClient skrclient.Client, _ Object,
	resources []*unstructured.Unstructured,
) error {
	for _, resource := range resources {
		if err := normaliseNamespace(resource, apimetav1.NamespaceDefault, skrClient); err != nil {
			if !meta.IsNoMatchError(err) {
				return err
			}
		}
	}
	return nil
}

func normaliseNamespace(obj *unstructured.Unstructured, defaultNamespace string,
	skrClient skrclient.Client,
) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	namespaced, err := isNamespaced(gvk, skrClient)
	if err != nil {
		return err
	}
	if namespaced {
		if obj.GetNamespace() == "" {
			obj.SetNamespace(defaultNamespace)
		}
	} else {
		if obj.GetNamespace() != "" {
			obj.SetNamespace("")
		}
	}
	return nil
}

func isNamespaced(gvk schema.GroupVersionKind, skrClient skrclient.Client) (bool, error) {
	mapper := skrClient.RESTMapper()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return false, fmt.Errorf("failed to get REST mapping for %s: %w", gvk.Kind, err)
	}
	return mapping.Scope.Name() == "namespace", nil
}
