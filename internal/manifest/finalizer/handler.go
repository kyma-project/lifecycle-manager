package finalizer

import (
	"context"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/labelsremoval"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/modulecr"
)

const (
	DefaultFinalizer               = "declarative.kyma-project.io/finalizer"
	CustomResourceManagerFinalizer = "resource.kyma-project.io/finalizer"
)

// RemoveRequiredFinalizers removes preconfigured finalizers, but not include modulecr.CustomResourceManagerFinalizer.
func RemoveRequiredFinalizers(manifest *v1beta2.Manifest) bool {
	finalizersToRemove := []string{DefaultFinalizer, labelsremoval.LabelRemovalFinalizer}

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
	labelRemovalFinalizerAdded := controllerutil.AddFinalizer(manifest, labelsremoval.LabelRemovalFinalizer)
	return defaultFinalizerAdded || labelRemovalFinalizerAdded
}

func EnsureCRFinalizer(ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest) error {
	if manifest.Spec.Resource == nil {
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
			client.FieldOwner(CustomResourceManagerFinalizer),
		); err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
		return modulecr.ErrRequeueRequired
	}
	return nil
}
