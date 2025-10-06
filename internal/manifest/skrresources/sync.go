package skrresources

import (
	"context"
	"errors"

	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
)

var ErrWarningResourceSyncStateDiff = errors.New("resource syncTarget state diff detected")

func SyncResources(ctx context.Context, skrClient client.Client, manifest *v1beta2.Manifest,
	target []*resource.Info,
) error {
	manifestStatus := manifest.GetStatus()

	managedFieldsCollector := NewManifestLogCollector(manifest, manifestclient.DefaultFieldOwner)

	if err := ConcurrentSSA(skrClient, manifestclient.DefaultFieldOwner, managedFieldsCollector).
		Run(ctx, target); err != nil {
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return err
	}

	oldSynced := manifestStatus.Synced
	newSynced := NewDefaultInfoToResourceConverter().InfosToResources(target)
	manifestStatus.Synced = newSynced

	if HasDiff(oldSynced, newSynced) {
		if manifest.GetDeletionTimestamp().IsZero() {
			manifest.SetStatus(
				manifestStatus.WithState(shared.StateProcessing).WithOperation(ErrWarningResourceSyncStateDiff.Error()),
			)
		} else if manifestStatus.State != shared.StateWarning {
			manifest.SetStatus(manifestStatus.WithState(shared.StateDeleting).
				WithOperation(ErrWarningResourceSyncStateDiff.Error()))
		}
		return ErrWarningResourceSyncStateDiff
	}
	return nil
}

func HasDiff(oldResources []shared.Resource, newResources []shared.Resource) bool {
	if len(oldResources) != len(newResources) {
		return true
	}
	countMap := map[string]bool{}
	for _, item := range oldResources {
		countMap[item.ID()] = true
	}
	for _, item := range newResources {
		if countMap[item.ID()] {
			countMap[item.ID()] = false
		}
	}
	for _, exists := range countMap {
		if exists {
			return true
		}
	}
	return false
}
