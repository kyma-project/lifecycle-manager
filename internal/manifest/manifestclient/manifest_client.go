package manifestclient

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
)

const DefaultFieldOwner client.FieldOwner = "declarative.kyma-project.io/applier"

type ManifestClient struct {
	client.Client
	event.Event
}

func NewManifestClient(event event.Event, kcpClient client.Client) *ManifestClient {
	return &ManifestClient{
		Event:  event,
		Client: kcpClient,
	}
}

func (m *ManifestClient) UpdateManifest(ctx context.Context, manifest *v1beta2.Manifest) error {
	err := m.Update(ctx, manifest)
	if err != nil {
		m.Warning(manifest, "UpdateObject", err)
		return fmt.Errorf("failed to update object: %w", err)
	}
	return nil
}

func (m *ManifestClient) PatchStatusIfDiffExist(ctx context.Context, manifest *v1beta2.Manifest,
	previousStatus shared.Status,
) error {
	if HasStatusDiff(manifest.GetStatus(), previousStatus) {
		resetNonPatchableField(manifest)
		err := m.Status().Patch(ctx, manifest, client.Apply, client.ForceOwnership, DefaultFieldOwner)
		if err != nil {
			m.Warning(manifest, "PatchStatus", err)
			return fmt.Errorf("failed to patch status: %w", err)
		}
	}

	return nil
}

func (m *ManifestClient) SsaSpec(ctx context.Context, obj client.Object) error {
	resetNonPatchableField(obj)
	err := m.Patch(ctx, obj, client.Apply, client.ForceOwnership, DefaultFieldOwner)
	if err != nil {
		m.Warning(obj, "PatchObject", err)
		return fmt.Errorf("failed to patch object: %w", err)
	}
	return nil
}

func HasStatusDiff(first, second shared.Status) bool {
	return first.State != second.State || first.Operation != second.Operation
}

func resetNonPatchableField(obj client.Object) {
	obj.SetUID("")
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
}
