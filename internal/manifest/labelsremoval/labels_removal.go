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
)

const LabelRemovalFinalizer = "label-removal-finalizer"

type ManifestAPIClient interface {
	UpdateManifest(ctx context.Context, manifest *v1beta2.Manifest) error
}

type ManagedLabelRemovalService struct {
	manifestClient ManifestAPIClient
}

func NewManagedLabelRemovalService(manifestClient ManifestAPIClient) *ManagedLabelRemovalService {
	return &ManagedLabelRemovalService{
		manifestClient: manifestClient,
	}
}

func (l *ManagedLabelRemovalService) RemoveManagedLabel(ctx context.Context,
	manifest *v1beta2.Manifest, skrClient client.Client, defaultCR *unstructured.Unstructured,
) error {
	if err := HandleLabelsRemovalFromResources(ctx, manifest, skrClient, defaultCR); err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(manifest, LabelRemovalFinalizer)
	return l.manifestClient.UpdateManifest(ctx, manifest)
}

func HandleLabelsRemovalFromResources(ctx context.Context, manifestCR *v1beta2.Manifest,
	skrClient client.Client, defaultCR *unstructured.Unstructured,
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

		if IsManagedLabelRemoved(obj) {
			if err := skrClient.Update(ctx, obj); err != nil {
				return fmt.Errorf("failed to update object: %w", err)
			}
		}
	}

	if defaultCR == nil {
		return nil
	}

	if IsManagedLabelRemoved(defaultCR) {
		if err := skrClient.Update(ctx, defaultCR); err != nil {
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

func IsManagedLabelRemoved(resource *unstructured.Unstructured) bool {
	labels := resource.GetLabels()
	_, managedByLabelExists := labels[shared.ManagedBy]
	if managedByLabelExists {
		delete(labels, shared.ManagedBy)
	}

	resource.SetLabels(labels)

	return managedByLabelExists
}
