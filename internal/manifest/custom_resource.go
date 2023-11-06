package manifest

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrWaitingForAsyncCustomResourceDeletion = errors.New(
		"deletion of custom resource was triggered and is now waiting to be completed")
	ErrWaitingForAsyncCustomResourceDefinitionDeletion = errors.New(
		"deletion of custom resource definition was triggered and is now waiting to be completed")
)

// PostRunCreateCR is a hook for creating the manifest default custom resource if not available in the cluster
// It is used to provide the controller with default data in the Runtime.
func PostRunCreateCR(
	ctx context.Context, skr declarativev2.Client, kcp client.Client, obj declarativev2.Object,
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
	err := skr.Create(ctx, resource, client.FieldOwner(declarativev2.CustomResourceManager))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	oMeta := &apimachinerymeta.PartialObjectMetadata{}
	oMeta.SetName(obj.GetName())
	oMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	oMeta.SetNamespace(obj.GetNamespace())
	oMeta.SetFinalizers(obj.GetFinalizers())
	if added := controllerutil.AddFinalizer(oMeta, declarativev2.CustomResourceManager); added {
		if err := kcp.Patch(
			ctx, oMeta, client.Apply, client.ForceOwnership, client.FieldOwner(declarativev2.CustomResourceManager),
		); err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
		return declarativev2.ErrRequeueRequired
	}
	return nil
}

// PreDeleteDeleteCR is a hook for deleting the manifest default custom resource if available in the cluster
// It is used to clean up the controller default data.
// It uses DeletePropagationBackground as it will return an error if the resource exists, even if deletion is triggered
// This leads to the reconciled resource immediately being requeued due to ErrWaitingForAsyncCustomResourceDeletion.
// In this case, the next time it will run into this delete function,
// it will either say that the resource is already being deleted (2xx) and retry or its no longer found.
// Then the finalizer is dropped, and we consider the CR removal successful.
func PreDeleteDeleteCR(
	ctx context.Context, skr declarativev2.Client, kcp client.Client, obj declarativev2.Object,
) error {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil
	}
	if manifest.Spec.Resource == nil {
		return nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	propagation := apimachinerymeta.DeletePropagationBackground
	err := skr.Delete(ctx, resource, &client.DeleteOptions{PropagationPolicy: &propagation})

	if !util.IsNotFound(err) {
		return nil
	}

	onCluster := manifest.DeepCopy()
	err = kcp.Get(ctx, client.ObjectKeyFromObject(obj), onCluster)
	if util.IsNotFound(err) {
		return fmt.Errorf("PreDeleteDeleteCR: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}
	if removed := controllerutil.RemoveFinalizer(onCluster, declarativev2.CustomResourceManager); removed {
		if err := kcp.Update(
			ctx, onCluster, client.FieldOwner(declarativev2.CustomResourceManager),
		); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
		return declarativev2.ErrRequeueRequired
	}
	return nil
}
