package skrresources

import (
	"context"
	"errors"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
)

var ErrWarningResourceSyncStateDiff = errors.New("resource syncTarget state diff detected")

func SyncResources(ctx context.Context, skrClient client.Client, manifest *v1beta2.Manifest,
	target []client.Object,
) error {
	manifestStatus := manifest.GetStatus()

	managedFieldsCollector := NewManifestLogCollector(manifest, fieldowners.DeclarativeApplier)

	if err := ConcurrentSSA(skrClient,
		fieldowners.DeclarativeApplier,
		managedFieldsCollector,
	).Run(ctx, target); err != nil {
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return err
	}

	oldSynced := manifestStatus.Synced
	newSynced := objectsToResources(target)
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

func objectsToResources(objs []client.Object) []shared.Resource {
	result := make([]shared.Resource, 0, len(objs))
	for _, obj := range objs {
		gvk := obj.GetObjectKind().GroupVersionKind()
		result = append(result, shared.Resource{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			GroupVersionKind: apimetav1.GroupVersionKind(
				schema.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
			),
		})
	}
	return result
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
