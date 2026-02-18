package finalizer

import (
	"context"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrRequeueRequired = errors.New("requeue required")

const (
	DefaultFinalizer               = "declarative.kyma-project.io/finalizer"
	CustomResourceManagerFinalizer = "resource.kyma-project.io/finalizer"
	LabelRemovalFinalizer          = "label-removal-finalizer"
)

// RemoveRequiredFinalizers removes preconfigured finalizers, but not include CustomResourceManagerFinalizer.
func RemoveRequiredFinalizers(manifest *v1beta2.Manifest) bool {
	finalizersToRemove := []string{DefaultFinalizer, LabelRemovalFinalizer}

	finalizerRemoved := false
	for _, f := range finalizersToRemove {
		if controllerutil.RemoveFinalizer(manifest, f) {
			finalizerRemoved = true
		}
	}
	return finalizerRemoved
}

func RemoveAllFinalizers(manifest *v1beta2.Manifest) bool {
	finalizerRemoved := false
	for _, f := range manifest.GetFinalizers() {
		if controllerutil.RemoveFinalizer(manifest, f) {
			finalizerRemoved = true
		}
	}
	return finalizerRemoved
}

func FinalizersUpdateRequired(manifest *v1beta2.Manifest) bool {
	defaultFinalizerAdded := controllerutil.AddFinalizer(manifest, DefaultFinalizer)
	labelRemovalFinalizerAdded := controllerutil.AddFinalizer(manifest, LabelRemovalFinalizer)
	return defaultFinalizerAdded || labelRemovalFinalizerAdded
}

func EnsureCRFinalizer(ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest) error {
	if manifest.Spec.Resource == nil || manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return nil
	}
	if !manifest.GetDeletionTimestamp().IsZero() {
		return nil
	}
	oMeta := &apimetav1.PartialObjectMetadata{}
	oMeta.SetName(manifest.GetName())
	oMeta.SetGroupVersionKind(manifest.GetObjectKind().GroupVersionKind())
	oMeta.SetNamespace(manifest.GetNamespace())
	oMeta.SetFinalizers(manifest.GetFinalizers())

	if added := controllerutil.AddFinalizer(oMeta, CustomResourceManagerFinalizer); added {
		if err := kcp.Patch(
			ctx, oMeta, client.Apply, client.ForceOwnership,
			fieldowners.CustomResourceFinalizer,
		); err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
		return ErrRequeueRequired
	}
	return nil
}

func RemoveCRFinalizer(ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest) error {
	if manifest.Spec.Resource == nil || manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return nil
	}
	onCluster := manifest.DeepCopy()

	if err := kcp.Get(ctx, client.ObjectKeyFromObject(manifest), onCluster); err != nil {
		// If the manifest is not found, we consider the finalizer removed.
		if util.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleteCR: %w", err)
	}

	if removed := controllerutil.RemoveFinalizer(onCluster, CustomResourceManagerFinalizer); removed {
		if err := kcp.Update(
			ctx, onCluster, fieldowners.CustomResourceFinalizer,
		); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
		return ErrRequeueRequired
	}
	return nil
}
