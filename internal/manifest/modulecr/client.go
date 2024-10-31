package modulecr

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type Client struct {
	client.Client
}

func NewClient(client client.Client) *Client {
	return &Client{
		client,
	}
}

func (c *Client) GetCR(ctx context.Context, manifest *v1beta2.Manifest) (*unstructured.Unstructured,
	error,
) {
	resourceCR := &unstructured.Unstructured{}
	name := manifest.Spec.Resource.GetName()
	namespace := manifest.Spec.Resource.GetNamespace()
	gvk := manifest.Spec.Resource.GroupVersionKind()

	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	if err := c.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		return nil, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}

	return resourceCR, nil
}

func (c *Client) CheckCRDeletion(ctx context.Context, manifestCR *v1beta2.Manifest) (bool,
	error,
) {
	if manifestCR.Spec.Resource == nil {
		return true, nil
	}

	resourceCR, err := c.GetCR(ctx, manifestCR)
	if err != nil {
		if util.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}

	return resourceCR == nil, nil
}

// RemoveModuleCR deletes the module CR if available in the cluster.
// It uses DeletePropagationBackground to delete module CR.
// Only if module CR is not found (indicated by NotFound error), it continues to remove Manifest finalizer,
// and we consider the CR removal successful.
func (c *Client) RemoveModuleCR(ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest) error {
	if !manifest.GetDeletionTimestamp().IsZero() {
		if err := c.deleteCR(ctx, manifest); err != nil {
			// we do not set a status here since it will be deleting if timestamp is set.
			manifest.SetStatus(manifest.GetStatus().WithErr(err))
			return err
		}
		if err := finalizer.RemoveCRFinalizer(ctx, kcp, manifest); err != nil {
			manifest.SetStatus(manifest.GetStatus().WithErr(err))
			return err
		}
	}

	return nil
}

func (c *Client) deleteCR(ctx context.Context, manifest *v1beta2.Manifest) error {
	if manifest.Spec.Resource == nil {
		return nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	propagation := apimetav1.DeletePropagationBackground
	err := c.Delete(ctx, resource, &client.DeleteOptions{PropagationPolicy: &propagation})

	if err != nil && !util.IsNotFound(err) {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}

	return nil
}

// SyncModuleCR sync the manifest default custom resource status in the cluster, if not available it created the resource.
// It is used to provide the controller with default data in the Runtime.
func (c *Client) SyncModuleCR(ctx context.Context, manifest *v1beta2.Manifest) (shared.State, error) {
	if manifest.Spec.Resource == nil {
		return "", nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	resource.SetLabels(collections.MergeMaps(resource.GetLabels(), map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}))

	if err := c.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil && util.IsNotFound(err) {
		if !manifest.GetDeletionTimestamp().IsZero() {
			return "", nil
		}
		if err := c.Create(ctx, resource,
			client.FieldOwner(finalizer.CustomResourceManagerFinalizer)); err != nil && !apierrors.IsAlreadyExists(err) {
			return "", fmt.Errorf("failed to create resource: %w", err)
		}
	}

	stateFromCR, _, err := unstructured.NestedString(resource.Object, "status", "state")
	return shared.State(stateFromCR), err
}
