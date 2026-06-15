package render

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

// NormaliseNamespace is a workaround for malformed resources, e.g. produced by
// bad charts or wrong type configs: namespaced resources are defaulted to
// defaultNamespace when missing one, and cluster-scoped resources have any
// stray namespace stripped. Whether a resource is namespaced is determined
// against the SKR's REST mapper.
func NormaliseNamespace(obj client.Object, defaultNamespace string, skrClient skrclient.Client) error {
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
