package remote

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	applyconfigurationsv1beta2 "github.com/kyma-project/lifecycle-manager/api/applyconfigurations/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
)

var (
	errModuleReleaseMetaCRDNotReady = errors.New("catalog sync: ModuleReleaseMeta CRD is not ready")
	errModuleReleaseMetaCleanup     = errors.New("catalog sync: Failed to delete ModuleReleaseMeta")
	errCatModuleReleaseMetaApply    = errors.New("catalog sync: Could not apply ModuleReleseMetas")
)

// moduleReleaseMetaConcurrentWorker performs ModuleReleaseMeta synchronization using multiple goroutines.
type moduleReleaseMetaConcurrentWorker struct {
	namespace  string
	patchDiff  func(ctx context.Context, diff *applyconfigurationsv1beta2.ModuleReleaseMetaApplyConfiguration) error
	deleteDiff func(ctx context.Context, obj *v1beta2.ModuleReleaseMeta) error
	createCRD  func(ctx context.Context) error
}

// newModuleReleaseMetaConcurrentWorker returns a new moduleReleaseMetaConcurrentWorker
// instance with default dependencies.
func newModuleReleaseMetaConcurrentWorker(
	kcpClient, skrClient client.Client,
	settings *Settings,
) *moduleReleaseMetaConcurrentWorker {
	patchDiffFn := func(
		ctx context.Context,
		diff *applyconfigurationsv1beta2.ModuleReleaseMetaApplyConfiguration,
	) error {
		return patchDiffModuleReleaseMeta(ctx, diff, skrClient, settings.SsaApplyOptions)
	}

	deleteDiffFn := func(ctx context.Context, obj *v1beta2.ModuleReleaseMeta) error {
		return deleteModuleReleaseMeta(ctx, obj, skrClient)
	}

	createCRDFn := func(ctx context.Context) error {
		return createModuleReleaseMetaCRDInRuntime(ctx, kcpClient, skrClient)
	}

	return &moduleReleaseMetaConcurrentWorker{
		namespace:  settings.Namespace,
		patchDiff:  patchDiffFn,
		deleteDiff: deleteDiffFn,
		createCRD:  createCRDFn,
	}
}

// SyncConcurrently synchronizes ModuleReleaseMetas from KCP to SKR.
// kcpModules are the ModuleReleaseMetas to be synced from the KCP cluster.
func (c *moduleReleaseMetaConcurrentWorker) SyncConcurrently(
	ctx context.Context,
	kcpModules []v1beta2.ModuleReleaseMeta,
) error {
	channelLength := len(kcpModules)
	results := make(chan error, channelLength)
	for kcpIndex := range kcpModules {
		go func() {
			applyConfig := prepareModuleReleaseMetaForSSA(&kcpModules[kcpIndex], c.namespace)
			results <- c.patchDiff(ctx, applyConfig)
		}()
	}
	var errs []error
	for range channelLength {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	// retry if ModuleReleaseMeta CRD is not existing in SKR cluster
	if containsCRDNotFoundError(errs) {
		if err := c.createCRD(ctx); err != nil {
			return err
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

func createModuleReleaseMetaCRDInRuntime(ctx context.Context, kcpClient client.Client, skrClient client.Client) error {
	return createCRDInRuntime(ctx, shared.ModuleReleaseMetaKind, errModuleReleaseMetaCRDNotReady, kcpClient, skrClient)
}

func prepareModuleReleaseMetaForSSA(moduleReleaseMeta *v1beta2.ModuleReleaseMeta, namespace string,
) *applyconfigurationsv1beta2.ModuleReleaseMetaApplyConfiguration {
	// It would be better to not change the namespace of the moduleReleaseMeta object here,
	// but at this moment it is required to perform comparisons between objects
	// later on - namespace is part of the object identity function.
	// This can be refactored.
	if namespace != "" {
		moduleReleaseMeta.SetNamespace(namespace)
	}

	var applyChannels []*applyconfigurationsv1beta2.ChannelVersionAssignmentApplyConfiguration
	if len(moduleReleaseMeta.Spec.Channels) > 0 {
		for _, channel := range moduleReleaseMeta.Spec.Channels {
			applyChannels = append(applyChannels, applyconfigurationsv1beta2.ChannelVersionAssignment().
				WithChannel(channel.Channel).
				WithVersion(channel.Version))
		}
	}

	var applyMandatory *applyconfigurationsv1beta2.MandatoryApplyConfiguration
	if moduleReleaseMeta.Spec.Mandatory != nil {
		applyMandatory = applyconfigurationsv1beta2.Mandatory().
			WithVersion(moduleReleaseMeta.Spec.Mandatory.Version)
	}

	specApplyConfig := applyconfigurationsv1beta2.ModuleReleaseMetaSpec().
		WithModuleName(moduleReleaseMeta.Spec.ModuleName).
		WithOcmComponentName(moduleReleaseMeta.Spec.OcmComponentName).
		WithChannels(applyChannels...).
		WithMandatory(applyMandatory)

	if moduleReleaseMeta.Spec.Beta { //nolint:staticcheck // backward compatibility
		//nolint:staticcheck // backward compatibility
		specApplyConfig = specApplyConfig.WithBeta(moduleReleaseMeta.Spec.Beta)
	}
	if moduleReleaseMeta.Spec.Internal { //nolint:staticcheck // backward compatibility
		//nolint:staticcheck // backward compatibility
		specApplyConfig = specApplyConfig.WithInternal(moduleReleaseMeta.Spec.Internal)
	}

	labelsApplyConfig := collections.MergeMapsSilent(moduleReleaseMeta.GetLabels(),
		map[string]string{shared.ManagedBy: shared.ManagedByLabelValue})

	res := applyconfigurationsv1beta2.ModuleReleaseMeta(
		moduleReleaseMeta.GetName(), moduleReleaseMeta.GetNamespace()).
		WithLabels(labelsApplyConfig).
		WithSpec(specApplyConfig)

	return res
}

func patchDiffModuleReleaseMeta(
	ctx context.Context,
	diff *applyconfigurationsv1beta2.ModuleReleaseMetaApplyConfiguration,
	skrClient client.Client,
	ssaApplyOptions *client.ApplyOptions,
) error {
	err := skrClient.Apply(ctx, diff, ssaApplyOptions)
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
