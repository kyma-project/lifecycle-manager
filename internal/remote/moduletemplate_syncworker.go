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
	errModuleTemplateCRDNotReady = errors.New("catalog sync: ModuleTemplate CRD is not ready")
	errModuleTemplateCleanup     = errors.New("catalog sync: Failed to delete obsolete ModuleTemplates")
	errCatModuleTemplatesApply   = errors.New("catalog sync: Could not apply ModuleTemplates")
)

// moduleTemplateConcurrentWorker performs ModuleTemplate synchronization using multiple goroutines.
type moduleTemplateConcurrentWorker struct {
	namespace  string
	patchDiff  func(ctx context.Context, diff *applyconfigurationsv1beta2.ModuleTemplateApplyConfiguration) error
	deleteDiff func(ctx context.Context, obj *v1beta2.ModuleTemplate) error
	createCRD  func(ctx context.Context) error
}

// newModuleTemplateConcurrentWorker returns a new moduleTemplateConcurrentWorker instance with default dependencies.
func newModuleTemplateConcurrentWorker(
	kcpClient, skrClient client.Client,
	settings *Settings,
) *moduleTemplateConcurrentWorker {
	patchDiffFn := func(ctx context.Context, diff *applyconfigurationsv1beta2.ModuleTemplateApplyConfiguration) error {
		return patchDiffModuleTemplate(ctx, diff, skrClient, settings.SsaApplyOptions)
	}

	deleteDiffFn := func(ctx context.Context, obj *v1beta2.ModuleTemplate) error {
		return deleteModuleTemplate(ctx, obj, skrClient)
	}

	createCRDFn := func(ctx context.Context) error {
		return createModuleTemplateCRDInRuntime(ctx, kcpClient, skrClient)
	}

	return &moduleTemplateConcurrentWorker{
		namespace:  settings.Namespace,
		patchDiff:  patchDiffFn,
		deleteDiff: deleteDiffFn,
		createCRD:  createCRDFn,
	}
}

// SyncConcurrently synchronizes ModuleTemplates from KCP to SKR.
// kcpModules are the ModuleTemplates to be synced from the KCP cluster.
func (c *moduleTemplateConcurrentWorker) SyncConcurrently(
	ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
) error {
	channelLength := len(kcpModules)
	results := make(chan error, channelLength)
	for kcpIndex := range kcpModules {
		go func() {
			applyConfig := prepareModuleTemplateForSSA(&kcpModules[kcpIndex], c.namespace)
			results <- c.patchDiff(ctx, applyConfig)
		}()
	}
	var errs []error
	for range channelLength {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	// retry if ModuleTemplate CRD is not existing in SKR cluster
	if containsCRDNotFoundError(errs) {
		if err := c.createCRD(ctx); err != nil {
			return err
		}
	}

	if len(errs) != 0 {
		errs = append(errs, errCatModuleTemplatesApply)
		return errors.Join(errs...)
	}
	return nil
}

// DeleteConcurrently deletes ModuleTemplates from SKR.
func (c *moduleTemplateConcurrentWorker) DeleteConcurrently(ctx context.Context,
	diffsToDelete []v1beta2.ModuleTemplate,
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
		errs = append(errs, errModuleTemplateCleanup)
		return errors.Join(errs...)
	}
	return nil
}

func createModuleTemplateCRDInRuntime(ctx context.Context, kcpClient client.Client, skrClient client.Client) error {
	return createCRDInRuntime(ctx, shared.ModuleTemplateKind, errModuleTemplateCRDNotReady, kcpClient, skrClient)
}

func prepareModuleTemplateForSSA(moduleTemplate *v1beta2.ModuleTemplate, namespace string,
) *applyconfigurationsv1beta2.ModuleTemplateApplyConfiguration {
	// It would be better to not change the namespace of the moduleTemplate object here,
	// but at this moment it is required to perform comparisons between objects
	// later on - namespace is part of the object identity function.
	// This can be refactored.
	if namespace != "" {
		moduleTemplate.SetNamespace(namespace)
	}

	labelsApplyConfig := collections.MergeMapsSilent(
		moduleTemplate.GetLabels(),
		map[string]string{
			shared.ManagedBy: shared.ManagedByLabelValue,
		},
	)

	var customStateCheckApplyConfig []**v1beta2.CustomStateCheck
	if len(moduleTemplate.Spec.CustomStateCheck) > 0 {
		customStateCheckApplyConfig = make([]**v1beta2.CustomStateCheck, len(moduleTemplate.Spec.CustomStateCheck))
		for i := range moduleTemplate.Spec.CustomStateCheck {
			customStateCheckApplyConfig[i] = &moduleTemplate.Spec.CustomStateCheck[i]
		}
	}

	var resourcesApplyConfig []*applyconfigurationsv1beta2.ResourceApplyConfiguration
	if len(moduleTemplate.Spec.Resources) > 0 {
		resourcesApplyConfig = make([]*applyconfigurationsv1beta2.ResourceApplyConfiguration,
			len(moduleTemplate.Spec.Resources))
		for i := range moduleTemplate.Spec.Resources {
			resourcesApplyConfig[i] = applyconfigurationsv1beta2.Resource().
				WithName(moduleTemplate.Spec.Resources[i].Name).
				WithLink(moduleTemplate.Spec.Resources[i].Link)
		}
	}

	var moduleInfoApplyConfig *applyconfigurationsv1beta2.ModuleInfoApplyConfiguration
	if moduleTemplate.Spec.Info != nil {
		moduleInfoApplyConfig = applyconfigurationsv1beta2.ModuleInfo().
			WithRepository(moduleTemplate.Spec.Info.Repository).
			WithDocumentation(moduleTemplate.Spec.Info.Documentation)
		for i := range moduleTemplate.Spec.Info.Icons {
			moduleInfoApplyConfig.WithIcons(
				applyconfigurationsv1beta2.ModuleIcon().
					WithName(moduleTemplate.Spec.Info.Icons[i].Name).
					WithLink(moduleTemplate.Spec.Info.Icons[i].Link),
			)
		}
	}

	var managerApplyConfig *applyconfigurationsv1beta2.ManagerApplyConfiguration
	if moduleTemplate.Spec.Manager != nil {
		managerApplyConfig = applyconfigurationsv1beta2.Manager().
			WithGroup(moduleTemplate.Spec.Manager.Group).
			WithVersion(moduleTemplate.Spec.Manager.Version).
			WithKind(moduleTemplate.Spec.Manager.Kind).
			WithNamespace(moduleTemplate.Spec.Manager.Namespace).
			WithName(moduleTemplate.Spec.Manager.Name)
	}

	specApplyConfig := applyconfigurationsv1beta2.ModuleTemplateSpec().
		WithChannel(moduleTemplate.Spec.Channel). //nolint: staticcheck // backwards compatibility
		WithVersion(moduleTemplate.Spec.Version).
		WithModuleName(moduleTemplate.Spec.ModuleName).
		WithMandatory(moduleTemplate.Spec.Mandatory).
		WithDescriptor(moduleTemplate.Spec.Descriptor).
		WithCustomStateCheck(customStateCheckApplyConfig...).
		WithResources(resourcesApplyConfig...).
		WithInfo(moduleInfoApplyConfig).
		WithAssociatedResources(moduleTemplate.Spec.AssociatedResources...).
		WithManager(managerApplyConfig).
		WithRequiresDowntime(moduleTemplate.Spec.RequiresDowntime)

	if moduleTemplate.Spec.Data != nil {
		specApplyConfig = specApplyConfig.WithData(*moduleTemplate.Spec.Data)
	}

	applyConfig := applyconfigurationsv1beta2.ModuleTemplate(moduleTemplate.GetName(), moduleTemplate.GetNamespace()).
		WithLabels(labelsApplyConfig).
		WithSpec(specApplyConfig)

	return applyConfig
}

func patchDiffModuleTemplate(
	ctx context.Context,
	diff *applyconfigurationsv1beta2.ModuleTemplateApplyConfiguration,
	skrClient client.Client,
	ssaPatchOptions *client.ApplyOptions,
) error {
	err := skrClient.Apply(ctx, diff, ssaPatchOptions)
	if err != nil {
		return fmt.Errorf("could not apply ModuleTemplate diff: %w", err)
	}
	return nil
}

func deleteModuleTemplate(
	ctx context.Context, diff *v1beta2.ModuleTemplate, skrClient client.Client,
) error {
	err := skrClient.Delete(ctx, diff)
	if err != nil {
		return fmt.Errorf("could not delete ModuleTemplate: %w", err)
	}
	return nil
}
