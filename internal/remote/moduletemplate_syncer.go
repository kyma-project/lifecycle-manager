package remote

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type syncWorker interface {
	SyncConcurrently(ctx context.Context, kcpModules []v1beta2.ModuleTemplate) error
	DeleteConcurrently(ctx context.Context, runtimeModules []v1beta2.ModuleTemplate) error
}

type syncWorkerFactory func(kcpClient, skrClient client.Client, settings *Settings) syncWorker

// syncer provides a top-level API for synchronizing ModuleTemplates from KCP to SKR.
// It expects a ready-to-use client to the KCP and SKR cluster.
type syncer struct {
	kcpClient           client.Client
	skrClient           client.Client
	settings            *Settings
	syncWorkerFactoryFn syncWorkerFactory
}

func newSyncer(kcpClient, skrClient client.Client, settings *Settings) *syncer {
	var syncWokerFactoryFn syncWorkerFactory = func(kcpClient, skrClient client.Client, settings *Settings) syncWorker {
		return newModuleTemplateConcurrentWorker(kcpClient, skrClient, settings)
	}

	return &syncer{
		kcpClient:           kcpClient,
		skrClient:           skrClient,
		settings:            settings,
		syncWorkerFactoryFn: syncWokerFactoryFn,
	}
}

// Sync first lists all currently available moduleTemplates in the Runtime.
// If there is a NoMatchError, it will attempt to install the CRD but only if there are available crs to copy.
// It will use a 2 stage process:
// 1. All ModuleTemplates that either have to be created based on the given Control Plane Templates
// 2. All ModuleTemplates that have to be removed as they were deleted form the Control Plane Templates
// It uses Server-Side-Apply Patches to optimize the turnaround required.
func (mts *syncer) SyncToSKR(ctx context.Context, kyma types.NamespacedName, kcpModules []v1beta2.ModuleTemplate) error {
	worker := mts.syncWorkerFactoryFn(mts.kcpClient, mts.skrClient, mts.settings)

	if err := worker.SyncConcurrently(ctx, kcpModules); err != nil {
		return err
	}

	runtimeModules := &v1beta2.ModuleTemplateList{}
	if err := mts.skrClient.List(ctx, runtimeModules); err != nil {
		// it can happen that the ModuleTemplate CRD is not caught during to apply if there are no modules to apply
		// if this is the case and there is no CRD there can never be any module templates to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("failed to list module templates from runtime: %w", err)
	}

	diffsToDelete := moduleTemplatesDiffFor(runtimeModules.Items).NotExistingIn(kcpModules)
	diffsToDelete = collections.FilterInPlace(diffsToDelete, isManagedByKcp)
	return worker.DeleteConcurrently(ctx, collections.Dereference(diffsToDelete))
}

// DeleteFromSKR deletes all ModuleTemplates managed by KLM from the SKR cluster.
func (mts *syncer) DeleteAllManaged(ctx context.Context, kyma types.NamespacedName) error {
	moduleTemplatesRuntime := &v1beta2.ModuleTemplateList{Items: []v1beta2.ModuleTemplate{}}
	if err := mts.skrClient.List(ctx, moduleTemplatesRuntime); err != nil {
		// if there is no CRD or no module template exists,
		// there can never be any module templates to delete
		if util.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to list module templates from skr: %w", err)
	}
	for i := range moduleTemplatesRuntime.Items {
		if isManagedByKcp(&moduleTemplatesRuntime.Items[i]) {
			if err := mts.skrClient.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil &&
				!util.IsNotFound(err) {
				return fmt.Errorf("failed to delete module template from skr: %w", err)
			}
		}
	}
	return nil
}

// moduleTemplatesDiffFor returns a diffCalc for ModuleTemplate objects.
func moduleTemplatesDiffFor(first []v1beta2.ModuleTemplate) *collections.DiffCalc[v1beta2.ModuleTemplate] {
	return &collections.DiffCalc[v1beta2.ModuleTemplate]{
		First: first,
		Identity: func(obj v1beta2.ModuleTemplate) string {
			return obj.Namespace + obj.Name
		},
	}
}

func isManagedByKcp(skrTemplate *v1beta2.ModuleTemplate) bool {
	for _, managedFieldEntry := range skrTemplate.ObjectMeta.ManagedFields {
		if managedFieldEntry.Manager == moduleCatalogSyncFieldManager {
			return true
		}
	}
	return false
}
