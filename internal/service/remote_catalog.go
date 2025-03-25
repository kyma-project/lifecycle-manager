package service

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

type RemoteCatalogService struct {
	kcpClient                         client.Client
	skrContextService                 *SKRContextService
	settings                          *remote.Settings
	moduleTemplateSyncAPIFactoryFn    moduleTemplateSyncAPIFactory
	moduleReleaseMetaSyncAPIFactoryFn moduleReleaseMetaSyncAPIFactory
}

// moduleTemplateSyncAPI encapsulates the top-level abstration for syncing module templates to a remote cluster.
type moduleTemplateSyncAPI interface {
	SyncToSKR(ctx context.Context, kcpModules []v1beta2.ModuleTemplate) error
	DeleteAllManaged(ctx context.Context) error
}

type moduleReleaseMetaSyncAPI interface {
	SyncToSKR(ctx context.Context, kcpModuleReleaseMeta []v1beta2.ModuleReleaseMeta) error
	DeleteAllManaged(ctx context.Context) error
}

// moduleTemplateSyncAPIFactory is a function that creates moduleTemplateSyncAPI instances.
type moduleTemplateSyncAPIFactory func(kcpClient, skrClient client.Client,
	settings *remote.Settings) moduleTemplateSyncAPI

// moduleReleaseMetaSyncAPIFactory is a function that creates moduleReleaseMetaSyncAPI instances.
type moduleReleaseMetaSyncAPIFactory func(kcpClient, skrClient client.Client,
	settings *remote.Settings) moduleReleaseMetaSyncAPI

func NewRemoteCatalogFromKyma(kcpClient client.Client, skrContextService *SKRContextService,
	remoteSyncNamespace string,
) *RemoteCatalogService {
	force := true
	return newRemoteCatalog(kcpClient, skrContextService,
		&remote.Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: remote.ModuleCatalogSyncFieldManager, Force: &force},
			Namespace:       remoteSyncNamespace,
		},
	)
}

func newRemoteCatalog(kcpClient client.Client, skrContextService *SKRContextService,
	settings *remote.Settings) *RemoteCatalogService {
	var moduleTemplateSyncerAPIFactoryFn moduleTemplateSyncAPIFactory = func(kcpClient, skrClient client.Client,
		settings *remote.Settings) moduleTemplateSyncAPI {
		return remote.NewModuleTemplateSyncer(kcpClient, skrClient, settings)
	}

	var moduleReleaseMetaSyncerAPIFactoryFn moduleReleaseMetaSyncAPIFactory = func(kcpClient, skrClient client.Client,
		settings *remote.Settings) moduleReleaseMetaSyncAPI {
		return remote.NewModuleReleaseMetaSyncer(kcpClient, skrClient, settings)
	}

	res := &RemoteCatalogService{
		kcpClient:                         kcpClient,
		skrContextService:                 skrContextService,
		settings:                          settings,
		moduleTemplateSyncAPIFactoryFn:    moduleTemplateSyncerAPIFactoryFn,
		moduleReleaseMetaSyncAPIFactoryFn: moduleReleaseMetaSyncerAPIFactoryFn,
	}

	return res
}

func (c *RemoteCatalogService) SyncModuleCatalog(ctx context.Context, kyma *v1beta2.Kyma) error {
	moduleReleaseMetas, err := c.GetModuleReleaseMetasToSync(ctx, kyma)
	if err != nil {
		return err
	}

	moduleTemplates, err := c.GetModuleTemplatesToSync(ctx, moduleReleaseMetas)
	if err != nil {
		return err
	}

	// https://github.com/kyma-project/lifecycle-manager/issues/2096
	// Remove this block after the migration to the new ModuleTemplate format is completed.
	oldModuleTemplate, err := c.GetOldModuleTemplatesToSync(ctx, kyma)
	if err != nil {
		return err
	}
	moduleTemplates = append(moduleTemplates, oldModuleTemplate...)

	return c.sync(ctx, kyma.GetNamespacedName(), moduleTemplates, moduleReleaseMetas)
}

func (c *RemoteCatalogService) sync(
	ctx context.Context,
	kyma types.NamespacedName,
	kcpModules []v1beta2.ModuleTemplate,
	kcpModuleReleaseMeta []v1beta2.ModuleReleaseMeta,
) error {
	skrContext, err := c.skrContextService.GetCache(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SKR context: %w", err)
	}

	moduleTemplates := c.moduleTemplateSyncAPIFactoryFn(c.kcpClient, skrContext.Client, c.settings)
	moduleReleaseMetas := c.moduleReleaseMetaSyncAPIFactoryFn(c.kcpClient, skrContext.Client, c.settings)

	mtErr := moduleTemplates.SyncToSKR(ctx, kcpModules)
	mrmErr := moduleReleaseMetas.SyncToSKR(ctx, kcpModuleReleaseMeta)

	return errors.Join(mtErr, mrmErr)
}

func (c *RemoteCatalogService) Delete(
	ctx context.Context,
	kyma types.NamespacedName,
) error {
	skrContext, err := c.skrContextService.GetCache(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SKR context: %w", err)
	}

	moduleTemplates := c.moduleTemplateSyncAPIFactoryFn(c.kcpClient, skrContext.Client, c.settings)
	return moduleTemplates.DeleteAllManaged(ctx)
}

// GetModuleReleaseMetasToSync returns a list of ModuleReleaseMetas that should be synced to the SKR.
// A ModuleReleaseMeta that is Beta or Internal is synced only if the Kyma is also Beta or Internal.
func (c *RemoteCatalogService) GetModuleReleaseMetasToSync(
	ctx context.Context,
	kyma *v1beta2.Kyma,
) ([]v1beta2.ModuleReleaseMeta, error) {
	moduleReleaseMetaList := &v1beta2.ModuleReleaseMetaList{}
	if err := c.kcpClient.List(ctx, moduleReleaseMetaList); err != nil {
		return nil, fmt.Errorf("failed to list ModuleReleaseMetas: %w", err)
	}

	moduleReleaseMetas := []v1beta2.ModuleReleaseMeta{}
	for _, moduleReleaseMeta := range moduleReleaseMetaList.Items {
		if remote.IsAllowedModuleReleaseMeta(moduleReleaseMeta, kyma) {
			moduleReleaseMetas = append(moduleReleaseMetas, moduleReleaseMeta)
		}
	}

	return moduleReleaseMetas, nil
}

// GetModuleTemplatesToSync returns a list of ModuleTemplates that should be synced to the SKR.
// A ModuleTemplate is synced if it is not mandatory and does not have sync disabled, and if
// it is referenced by a ModuleReleaseMeta that is synced.
func (c *RemoteCatalogService) GetModuleTemplatesToSync(
	ctx context.Context,
	moduleReleaseMetas []v1beta2.ModuleReleaseMeta,
) ([]v1beta2.ModuleTemplate, error) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := c.kcpClient.List(ctx, moduleTemplateList); err != nil {
		return nil, fmt.Errorf("failed to list ModuleTemplates: %w", err)
	}

	return FilterAllowedModuleTemplates(moduleTemplateList.Items, moduleReleaseMetas), nil
}

// FilterAllowedModuleTemplates filters out ModuleTemplates that are not allowed.
// A ModuleTemplate is allowed if it is not mandatory, does not have sync disabled, and if
// it is referenced by a ModuleReleaseMeta that is synced.
func FilterAllowedModuleTemplates(
	moduleTemplates []v1beta2.ModuleTemplate,
	moduleReleaseMetas []v1beta2.ModuleReleaseMeta,
) []v1beta2.ModuleTemplate {
	moduleTemplatesToSync := map[string]bool{}
	for _, moduleReleaseMeta := range moduleReleaseMetas {
		for _, channel := range moduleReleaseMeta.Spec.Channels {
			moduleTemplatesToSync[formatModuleName(moduleReleaseMeta.Spec.ModuleName, channel.Version)] = true
		}
	}

	filteredModuleTemplates := []v1beta2.ModuleTemplate{}
	for _, moduleTemplate := range moduleTemplates {
		if moduleTemplate.IsMandatory() {
			continue
		}

		if moduleTemplate.HasSyncDisabled() {
			continue
		}

		if _, found := moduleTemplatesToSync[formatModuleName(moduleTemplate.Spec.ModuleName,
			moduleTemplate.Spec.Version)]; found {
			filteredModuleTemplates = append(filteredModuleTemplates, moduleTemplate)
		}
	}

	return filteredModuleTemplates
}

// https://github.com/kyma-project/lifecycle-manager/issues/2096
// Remove this function after the migration to the new ModuleTemplate format is completed.
func (c *RemoteCatalogService) GetOldModuleTemplatesToSync(
	ctx context.Context,
	kyma *v1beta2.Kyma,
) ([]v1beta2.ModuleTemplate, error) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := c.kcpClient.List(ctx, moduleTemplateList); err != nil {
		return nil, fmt.Errorf("failed to list ModuleTemplates: %w", err)
	}

	moduleTemplates := []v1beta2.ModuleTemplate{}
	for _, moduleTemplate := range moduleTemplateList.Items {
		if moduleTemplate.Spec.Channel == "" {
			continue
		}

		if moduleTemplate.SyncEnabled(kyma.IsBeta(), kyma.IsInternal()) {
			moduleTemplates = append(moduleTemplates, moduleTemplate)
		}
	}

	return moduleTemplates, nil
}

func formatModuleName(moduleName, version string) string {
	return fmt.Sprintf("%s-%s", moduleName, version)
}
