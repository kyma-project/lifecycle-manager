package v1beta1

import (
	"context"
	"errors"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

const CustomResourceManager = "resource.kyma-project.io/finalizer"

var ErrWaitingForAsyncCustomResourceDeletion = errors.New(
	"deletion of custom resource was triggered and is now waiting to be completed",
)

// PostRunCreateCR is a hook for creating the manifest default custom resource if not available in the cluster
// It is used to provide the controller with default data in the Runtime.
func PostRunCreateCR(
	ctx context.Context, skr declarative.Client, kcp client.Client, obj declarative.Object,
) error {
	manifest := obj.(*manifestv1beta2.Manifest)
	if manifest.Spec.Resource == nil {
		return nil
	}
	resource := manifest.Spec.Resource.DeepCopy()

	if err := skr.Create(
		ctx, resource, client.FieldOwner(CustomResourceManager),
	); err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	oMeta := &v1.PartialObjectMetadata{}
	oMeta.SetName(obj.GetName())
	oMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	oMeta.SetNamespace(obj.GetNamespace())
	oMeta.SetFinalizers(obj.GetFinalizers())
	if added := controllerutil.AddFinalizer(oMeta, CustomResourceManager); added {
		if err := kcp.Patch(
			ctx, oMeta, client.Apply, client.ForceOwnership, client.FieldOwner(CustomResourceManager),
		); err != nil {
			return err
		}
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
	ctx context.Context, skr declarative.Client, kcp client.Client, obj declarative.Object,
) error {
	manifest := obj.(*manifestv1beta2.Manifest)
	if manifest.Spec.Resource == nil {
		return nil
	}
	resource := manifest.Spec.Resource.DeepCopy()

	propagation := v1.DeletePropagationBackground
	err := skr.Delete(ctx, resource, &client.DeleteOptions{PropagationPolicy: &propagation})

	if err == nil {
		return ErrWaitingForAsyncCustomResourceDeletion
	}

	if !k8serrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		return err
	}

	onCluster := manifest.DeepCopy()
	err = kcp.Get(ctx, client.ObjectKeyFromObject(obj), onCluster)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if removed := controllerutil.RemoveFinalizer(onCluster, CustomResourceManager); removed {
		if err := kcp.Update(
			ctx, onCluster, client.FieldOwner(CustomResourceManager),
		); err != nil {
			return err
		}
	}
	return nil
}
