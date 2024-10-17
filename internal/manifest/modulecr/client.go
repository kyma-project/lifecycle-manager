package modulecr

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const CustomResourceManagerFinalizer = "resource.kyma-project.io/finalizer"

var ErrRequeueRequired = errors.New("requeue required")

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
		if err := c.DeleteCR(ctx, kcp, manifest); err != nil {
			// we do not set a status here since it will be deleting if timestamp is set.
			manifest.SetStatus(manifest.GetStatus().WithErr(err))
			return err
		}
	}

	return nil
}

// DeleteCR deletes the module CR if available in the cluster.
// It uses DeletePropagationBackground to delete module CR.
// Only if module CR is not found (indicated by NotFound error), it continues to remove Manifest finalizer,
// and we consider the CR removal successful.
func (c *Client) DeleteCR(
	ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest,
) error {
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
	err = kcp.Get(ctx, client.ObjectKeyFromObject(manifest), onCluster)
	if util.IsNotFound(err) {
		return fmt.Errorf("DeleteCR: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}
	if removed := controllerutil.RemoveFinalizer(onCluster, CustomResourceManagerFinalizer); removed {
		if err := kcp.Update(
			ctx, onCluster, client.FieldOwner(CustomResourceManagerFinalizer),
		); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
		return ErrRequeueRequired
	}
	return nil
}

// CreateCR creates the manifest default custom resource if not available in the cluster
// It is used to provide the controller with default data in the Runtime.
func (c *Client) CreateCR(
	ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest,
) error {
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

	err := c.Create(ctx, resource, client.FieldOwner(CustomResourceManagerFinalizer))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	oMeta := &apimetav1.PartialObjectMetadata{}
	oMeta.SetName(manifest.GetName())
	oMeta.SetGroupVersionKind(manifest.GetObjectKind().GroupVersionKind())
	oMeta.SetNamespace(manifest.GetNamespace())
	oMeta.SetFinalizers(manifest.GetFinalizers())

	if added := controllerutil.AddFinalizer(oMeta, CustomResourceManagerFinalizer); added {
		if err := kcp.Patch(
			ctx, oMeta, client.Apply, client.ForceOwnership,
			client.FieldOwner(CustomResourceManagerFinalizer),
		); err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
		return ErrRequeueRequired
	}
	return nil
}
