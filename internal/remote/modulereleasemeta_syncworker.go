package remote

import (
	"context"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

var (
	errModuleReleaseMetaCleanup  = errors.New("catalog sync: Failed to delete ModuleReleaseMeta")
	errCatModuleReleaseMetaApply = errors.New("catalog sync: Could not apply ModuleReleaseMetas")
)

// moduleReleaseMetaConcurrentWorker performs ModuleReleaseMeta synchronization using multiple goroutines.
type moduleReleaseMetaConcurrentWorker struct {
	namespace  string
	patchDiff  func(ctx context.Context, obj *v1beta2.ModuleReleaseMeta) error
	deleteDiff func(ctx context.Context, obj *v1beta2.ModuleReleaseMeta) error
}

// newModuleReleaseMetaConcurrentWorker returns a new moduleReleaseMetaConcurrentWorker
// instance with default dependencies.
func newModuleReleaseMetaConcurrentWorker(
	skrClient client.Client,
	settings *Settings,
) *moduleReleaseMetaConcurrentWorker {
	patchDiffFn := func(ctx context.Context, obj *v1beta2.ModuleReleaseMeta) error {
		return patchDiffModuleReleaseMeta(ctx, obj, skrClient, settings.SSAPatchOptions)
	}

	deleteDiffFn := func(ctx context.Context, obj *v1beta2.ModuleReleaseMeta) error {
		return deleteModuleReleaseMeta(ctx, obj, skrClient)
	}

	return &moduleReleaseMetaConcurrentWorker{
		namespace:  settings.Namespace,
		patchDiff:  patchDiffFn,
		deleteDiff: deleteDiffFn,
	}
}

// SyncConcurrently synchronizes ModuleReleaseMetas from KCP to SKR.
// kcpModules are the ModuleReleaseMetas to be synced from the KCP cluster.
// CRDs are expected to be installed beforehand by the central CRD sync. If the SKR API server reports
// a no-match error for the ModuleReleaseMeta kind, the error is propagated so the controller can requeue
// with backoff and re-enter the reconciliation where CRDs are tried to be installed again.
func (c *moduleReleaseMetaConcurrentWorker) SyncConcurrently(
	ctx context.Context,
	kcpModules []v1beta2.ModuleReleaseMeta,
) error {
	channelLength := len(kcpModules)
	results := make(chan error, channelLength)
	for kcpIndex := range kcpModules {
		go func() {
			prepareModuleReleaseMetaForSSA(&kcpModules[kcpIndex], c.namespace)
			results <- c.patchDiff(ctx, &kcpModules[kcpIndex])
		}()
	}
	var errs []error
	for range channelLength {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		errs = append(errs, errCatModuleReleaseMetaApply)
		return errors.Join(errs...)
	}
	return nil
}

// DeleteConcurrently deletes ModuleReleaseMetas from SKR.
func (c *moduleReleaseMetaConcurrentWorker) DeleteConcurrently(ctx context.Context,
	diffsToDelete []v1beta2.ModuleReleaseMeta,
) error {
	channelLength := len(diffsToDelete)
	results := make(chan error, channelLength)
	for _, diff := range diffsToDelete {
		go func() {
			results <- c.deleteDiff(ctx, &diff)
		}()
	}
	var errs []error
	for range channelLength {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		errs = append(errs, errModuleReleaseMetaCleanup)
		return errors.Join(errs...)
	}
	return nil
}

func prepareModuleReleaseMetaForSSA(moduleReleaseMeta *v1beta2.ModuleReleaseMeta, namespace string) {
	moduleReleaseMeta.SetResourceVersion("")
	moduleReleaseMeta.SetUID("")
	moduleReleaseMeta.SetManagedFields([]apimetav1.ManagedFieldsEntry{})
	moduleReleaseMeta.SetLabels(collections.MergeMapsSilent(moduleReleaseMeta.GetLabels(), map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}))

	if namespace != "" {
		moduleReleaseMeta.SetNamespace(namespace)
	}
}

func patchDiffModuleReleaseMeta(
	ctx context.Context,
	diff *v1beta2.ModuleReleaseMeta,
	skrClient client.Client,
	ssaPatchOptions *client.PatchOptions,
) error {
	err := skrClient.Patch(
		//nolint: staticcheck // issues: #2706, #2707
		ctx, diff, client.Apply, ssaPatchOptions,
	)
	if err != nil {
		return fmt.Errorf("could not apply ModuleReleaseMeta diff: %w", err)
	}
	return nil
}

func deleteModuleReleaseMeta(
	ctx context.Context, diff *v1beta2.ModuleReleaseMeta, skrClient client.Client,
) error {
	err := skrClient.Delete(ctx, diff)
	if err != nil {
		return fmt.Errorf("could not delete ModuleReleaseMeta: %w", err)
	}
	return nil
}
