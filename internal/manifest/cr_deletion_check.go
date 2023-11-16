package manifest

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

// NewModuleCRDeletionCheck creates a check that verifies that the Resource CR in the remote cluster is deleted.
func NewModuleCRDeletionCheck() *ModuleCRDeletionCheck {
	return &ModuleCRDeletionCheck{}
}

type ModuleCRDeletionCheck struct{}

func (c *ModuleCRDeletionCheck) Run(
	ctx context.Context,
	clnt client.Client,
	obj declarativev2.Object,
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

	resourceCR := &unstructured.Unstructured{}
	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	if err := clnt.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		if util.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}
	return false, nil
}
