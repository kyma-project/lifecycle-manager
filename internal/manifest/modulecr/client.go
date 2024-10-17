package modulecr

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
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

func (c *Client) RemoveModuleCR(ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest) error {
	if !manifest.GetDeletionTimestamp().IsZero() {
		if err := c.preDeleteDeleteCR(ctx, kcp, manifest); err != nil {
			// we do not set a status here since it will be deleting if timestamp is set.
			manifest.SetStatus(manifest.GetStatus().WithErr(err))
			return err
		}
	}

	return nil
}

// preDeleteDeleteCR is a hook for deleting the module CR if available in the cluster.
// It uses DeletePropagationBackground to delete module CR.
// Only if module CR is not found (indicated by NotFound error), it continues to remove Manifest finalizer,
// and we consider the CR removal successful.
func (c *Client) preDeleteDeleteCR(
	ctx context.Context, kcp client.Client, obj declarativev2.Object,
) error {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil
	}
	if manifest.Spec.Resource == nil {
		return nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	propagation := apimetav1.DeletePropagationBackground
	err := c.Delete(ctx, resource, &client.DeleteOptions{PropagationPolicy: &propagation})

	if !util.IsNotFound(err) {
		return nil
	}

	onCluster := manifest.DeepCopy()
	err = kcp.Get(ctx, client.ObjectKeyFromObject(obj), onCluster)
	if util.IsNotFound(err) {
		return fmt.Errorf("preDeleteDeleteCR: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}
	if removed := controllerutil.RemoveFinalizer(onCluster, declarativev2.CustomResourceManagerFinalizer); removed {
		if err := kcp.Update(
			ctx, onCluster, client.FieldOwner(declarativev2.CustomResourceManagerFinalizer),
		); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
		return declarativev2.ErrRequeueRequired
	}
	return nil
}

// PostRunCreateCR is a hook for creating the manifest default custom resource if not available in the cluster
// It is used to provide the controller with default data in the Runtime.
func (c *Client) PostRunCreateCR(
	ctx context.Context, kcp client.Client, obj declarativev2.Object,
) error {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil
	}
	if manifest.Spec.Resource == nil {
		return nil
	}
	if !manifest.GetDeletionTimestamp().IsZero() {
		return nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	resource.SetLabels(collections.MergeMaps(resource.GetLabels(), map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}))

	err := c.Create(ctx, resource, client.FieldOwner(declarativev2.CustomResourceManagerFinalizer))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	oMeta := &apimetav1.PartialObjectMetadata{}
	oMeta.SetName(obj.GetName())
	oMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	oMeta.SetNamespace(obj.GetNamespace())
	oMeta.SetFinalizers(obj.GetFinalizers())

	if added := controllerutil.AddFinalizer(oMeta, declarativev2.CustomResourceManagerFinalizer); added {
		if err := kcp.Patch(
			ctx, oMeta, client.Apply, client.ForceOwnership,
			client.FieldOwner(declarativev2.CustomResourceManagerFinalizer),
		); err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
		return declarativev2.ErrRequeueRequired
	}
	return nil
}
