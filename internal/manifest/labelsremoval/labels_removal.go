package labelsremoval

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
)

const LabelRemovalFinalizer = "label-removal-finalizer"

func HandleLabelsRemovalFinalizerForUnmanagedModule(ctx context.Context,
	manifest *v1beta2.Manifest, skrClient client.Client, manifestClnt manifestclient.ManifestClient,
	defaultCR *unstructured.Unstructured,
) error {
	if err := HandleLabelsRemovalFromResources(ctx, manifest, skrClient, defaultCR); err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(manifest, LabelRemovalFinalizer)
	return manifestClnt.UpdateManifest(ctx, manifest)
}

func HandleLabelsRemovalFromResources(ctx context.Context, manifestCR *v1beta2.Manifest,
	skrClient client.Client, defaultCR *unstructured.Unstructured,
) error {
	for _, res := range manifestCR.Status.Synced {
		objectKey := client.ObjectKey{
			Name:      res.Name,
			Namespace: res.Namespace,
		}
		gvk := schema.GroupVersionKind{
			Group:   res.Group,
			Version: res.Version,
			Kind:    res.Kind,
		}

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		if err := skrClient.Get(ctx, objectKey, obj); err != nil {
			return fmt.Errorf("failed to get resource, %w", err)
		}

		if NeedsUpdateAfterLabelRemoval(obj) {
			if err := skrClient.Update(ctx, obj); err != nil {
				return fmt.Errorf("failed to update object: %w", err)
			}
		}
	}

	if defaultCR == nil {
		return nil
	}

	if NeedsUpdateAfterLabelRemoval(defaultCR) {
		if err := skrClient.Update(ctx, defaultCR); err != nil {
			return fmt.Errorf("failed to update object: %w", err)
		}
	}

	return nil
}

func NeedsUpdateAfterLabelRemoval(resource *unstructured.Unstructured) bool {
	labels := resource.GetLabels()
	_, managedByLabelExists := labels[shared.ManagedBy]
	if managedByLabelExists {
		delete(labels, shared.ManagedBy)
	}
	_, watchedByLabelExists := labels[shared.WatchedByLabel]
	if watchedByLabelExists {
		delete(labels, shared.WatchedByLabel)
	}

	resource.SetLabels(labels)

	return watchedByLabelExists || managedByLabelExists
}
