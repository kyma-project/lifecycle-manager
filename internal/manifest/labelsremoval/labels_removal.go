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
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
)

type ManifestAPIClient interface {
	UpdateManifest(ctx context.Context, manifest *v1beta2.Manifest) error
}

type ManagedByLabelRemovalService struct {
	manifestClient ManifestAPIClient
}

func NewManagedByLabelRemovalService(manifestClient ManifestAPIClient) *ManagedByLabelRemovalService {
	return &ManagedByLabelRemovalService{
		manifestClient: manifestClient,
	}
}

func (l *ManagedByLabelRemovalService) RemoveManagedByLabel(ctx context.Context,
	manifest *v1beta2.Manifest,
	skrClient client.Client,
) error {
	if err := removeFromSyncedResources(ctx, manifest, skrClient); err != nil {
		return fmt.Errorf("failed to remove %s label from one or more resources: %w",
			shared.ManagedBy, err)
	}

	controllerutil.RemoveFinalizer(manifest, finalizer.LabelRemovalFinalizer)
	return l.manifestClient.UpdateManifest(ctx, manifest)
}

func removeFromSyncedResources(ctx context.Context, manifestCR *v1beta2.Manifest,
	skrClient client.Client,
) error {
	for _, res := range manifestCR.Status.Synced {
		objectKey := client.ObjectKey{
			Name:      res.Name,
			Namespace: res.Namespace,
		}

		obj := constructResource(res)
		if err := skrClient.Get(ctx, objectKey, obj); err != nil {
			return fmt.Errorf("failed to get resource, %w", err)
		}

		if err := removeFromObject(ctx, obj, skrClient); err != nil {
			return err
		}
	}

	return nil
}

func removeFromObject(ctx context.Context, obj *unstructured.Unstructured, skrClient client.Client) error {
	if removeManagedLabel(obj) {
		if err := skrClient.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to update object: %w", err)
		}
	}

	return nil
}

func constructResource(resource shared.Resource) *unstructured.Unstructured {
	gvk := schema.GroupVersionKind{
		Group:   resource.Group,
		Version: resource.Version,
		Kind:    resource.Kind,
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	return obj
}

func removeManagedLabel(resource *unstructured.Unstructured) bool {
	labels := resource.GetLabels()
	_, managedByLabelExists := labels[shared.ManagedBy]
	if managedByLabelExists {
		delete(labels, shared.ManagedBy)
	}

	resource.SetLabels(labels)

	return managedByLabelExists
}
