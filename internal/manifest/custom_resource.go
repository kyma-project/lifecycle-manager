package manifest

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
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
	labels := resource.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[shared.ManagedBy] = shared.ManagedByLabelValue
	resource.SetLabels(labels)

	err := skr.Create(ctx, resource, client.FieldOwner(declarativev2.CustomResourceManagerFinalizer))
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

// PreDeleteDeleteCR is a hook for deleting the module CR if available in the cluster.
// It uses DeletePropagationBackground to delete module CR.
// Only if module CR is not found (indicated by NotFound error), it continues to remove Manifest finalizer,
// and we consider the CR removal successful.
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
	propagation := apimetav1.DeletePropagationBackground
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
