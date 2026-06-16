package manifest

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

// ResourceList provides convenience methods for comparing collections of
// synced resources against the freshly rendered target list.
type ResourceList []shared.Resource

// Difference returns resources from r that are not present in target (by identity).
func (r ResourceList) Difference(target []client.Object) ResourceList {
	targetIDs := make(map[string]struct{}, len(target))
	for _, obj := range target {
		targetIDs[objectID(obj)] = struct{}{}
	}
	var diff ResourceList
	for _, res := range r {
		if _, found := targetIDs[res.ID()]; !found {
			diff = append(diff, res)
		}
	}
	return diff
}

// objectID returns a stable identity string for a client.Object matching shared.Resource.ID() format.
func objectID(obj client.Object) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return strings.Join([]string{obj.GetNamespace(), obj.GetName(), gvk.Group, gvk.Version, gvk.Kind}, "/")
}
