package remote

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

// moduleReleaseMetaSyncWorker is an interface for worker synchronizing ModuleReleaseMetas from KCP to SKR.
type moduleReleaseMetaSyncWorker interface {
	SyncConcurrently(ctx context.Context, kcpModules []v1beta2.ModuleReleaseMeta) error
	DeleteConcurrently(ctx context.Context, runtimeModules []v1beta2.ModuleReleaseMeta) error
}

// mrmSyncWorkerFactory is a factory function for creating new moduleReleaseMetaSyncWorker instance.
type mrmSyncWorkerFactory func(kcpClient, skrClient client.Client, settings *Settings) moduleReleaseMetaSyncWorker

// moduleReleaseMetaSyncer provides a top-level API for synchronizing ModuleReleaseMetas from KCP to SKR.
// It expects a ready-to-use client to the KCP and SKR cluster.
type moduleReleaseMetaSyncer struct {
	kcpClient           client.Client
	skrClient           client.Client
	settings            *Settings
	syncWorkerFactoryFn mrmSyncWorkerFactory
}

func newModuleReleaseMetaSyncer(kcpClient, skrClient client.Client, settings *Settings) *moduleReleaseMetaSyncer {
	var syncWokerFactoryFn mrmSyncWorkerFactory = func(kcpClient,
		skrClient client.Client, settings *Settings,
	) moduleReleaseMetaSyncWorker {
		return newModuleReleaseMetaConcurrentWorker(kcpClient, skrClient, settings)
	}

	return &moduleReleaseMetaSyncer{
		kcpClient:           kcpClient,
		skrClient:           skrClient,
		settings:            settings,
		syncWorkerFactoryFn: syncWokerFactoryFn,
	}
}

// SyncToSKR first lists all currently available ModuleReleaseMetas in the Runtime.
// If there is a NoMatchError, it will attempt to install the CRD but only if there are available crs to copy.
// It will use a 2 stage process:
// 1. All ModuleReleaseMeta that have to be created based on the ModuleReleaseMetas existing in the Control Plane.
// 2. All ModuleReleaseMeta that have to be removed as they are not existing in the Control Plane.
// It uses Server-Side-Apply Patches to optimize the turnaround required.
func (mts *moduleReleaseMetaSyncer) SyncToSKR(
	ctx context.Context,
	kcpModuleReleases []v1beta2.ModuleReleaseMeta,
) error {
	worker := mts.syncWorkerFactoryFn(mts.kcpClient, mts.skrClient, mts.settings)

	if err := worker.SyncConcurrently(ctx, kcpModuleReleases); err != nil {
		return err
	}

	runtimeModuleReleases := &v1beta2.ModuleReleaseMetaList{}
	if err := mts.skrClient.List(ctx, runtimeModuleReleases); err != nil {
		// it can happen that the ModuleReleaseMeta CRD is not caught during to apply if there are no objects to apply
		// if this is the case and there is no CRD there can never be any ModuleReleaseMetas to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("failed to list ModuleReleaseMetas from runtime: %w", err)
	}

	diffsToDelete := moduleReleaseMetasDiffFor(runtimeModuleReleases.Items).NotExistingIn(kcpModuleReleases)
	diffsToDelete = collections.FilterInPlace(diffsToDelete, isModuleReleaseMetaManagedByKcp)
	return worker.DeleteConcurrently(ctx, collections.Dereference(diffsToDelete))
}

// DeleteAllManaged deletes all ModuleReleaseMetas managed by KLM from the SKR cluster.
func (mts *moduleReleaseMetaSyncer) DeleteAllManaged(ctx context.Context) error {
	moduleReleaseMetasRuntime := &v1beta2.ModuleReleaseMetaList{Items: []v1beta2.ModuleReleaseMeta{}}
	if err := mts.skrClient.List(ctx, moduleReleaseMetasRuntime); err != nil {
		// if there is no CRD or no ModuleReleaseMeta exists,
		// there can never be any ModuleReleaseMeta to delete
		if util.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to list ModuleReleaseMeta from skr: %w", err)
	}
	for i := range moduleReleaseMetasRuntime.Items {
		if isModuleReleaseMetaManagedByKcp(&moduleReleaseMetasRuntime.Items[i]) {
			if err := mts.skrClient.Delete(ctx, &moduleReleaseMetasRuntime.Items[i]); err != nil &&
				!util.IsNotFound(err) {
				return fmt.Errorf("failed to delete ModuleReleaseMeta from skr: %w", err)
			}
		}
	}
	return nil
}

// moduleReleaseMetasDiffFor returns a diffCalc for ModuleReleaseMeta objects.
func moduleReleaseMetasDiffFor(first []v1beta2.ModuleReleaseMeta) *collections.DiffCalc[v1beta2.ModuleReleaseMeta] {
	return &collections.DiffCalc[v1beta2.ModuleReleaseMeta]{
		First: first,
		Identity: func(obj v1beta2.ModuleReleaseMeta) string {
			return obj.Namespace + obj.Name
		},
	}
}

func isModuleReleaseMetaManagedByKcp(skrObject *v1beta2.ModuleReleaseMeta) bool {
	for _, managedFieldEntry := range skrObject.ManagedFields {
		if managedFieldEntry.Manager == moduleCatalogSyncFieldManager {
			return true
		}
	}
	return false
}
