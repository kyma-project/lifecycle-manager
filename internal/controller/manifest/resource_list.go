package manifest

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		targetIDs[resourceIDFromObject(obj)] = struct{}{}
	}
	var diff ResourceList
	for _, res := range r {
		if _, found := targetIDs[res.ID()]; !found {
			diff = append(diff, res)
		}
	}
	return diff
}

// resourceIDFromObject returns shared.Resource.ID() for a client.Object so
// that identity comparison stays consistent with ResourceList entries even
// when the format of shared.Resource.ID() changes.
func resourceIDFromObject(obj client.Object) string {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return shared.Resource{
		GroupVersionKind: apimetav1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}.ID()
}
