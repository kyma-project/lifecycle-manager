package manifest

import (
	"context"

	"github.com/pkg/errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewCustomResourceDeletionCheck creates a check that verifies that the Resource CR in the remote cluster is deleted.
func NewCustomResourceDeletionCheck() *CRDeletionCheck {
	return &CRDeletionCheck{}
}

type CRDeletionCheck struct{}

func (c *CRDeletionCheck) Run(
	ctx context.Context,
	clnt client.Client,
	obj declarative.Object,
) (bool, error) {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return false, v1beta2.ErrTypeAssertManifest
	}
	if manifest.Spec.Resource == nil {
		return true, nil
	}

	name := manifest.Spec.Resource.GetName()
	namespace := manifest.Spec.Resource.GetNamespace()
	gvk := manifest.Spec.Resource.GroupVersionKind()

	resourceCR := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
	}}
	if err := clnt.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		if util.IsNotFound(err) {
			return true, nil
		}
		return false, errors.Wrap(err, "failed to fetch default resource CR")
	}
	if err := clnt.Delete(ctx, resourceCR); err != nil {
		return false, errors.Wrap(err, "failed to delete resource CR")
	}
	return false, nil
}
