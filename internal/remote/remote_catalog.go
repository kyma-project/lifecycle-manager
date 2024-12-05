package remote

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const moduleCatalogSyncFieldManager = "catalog-sync"

type Settings struct {
	// this namespace flag can be used to override the namespace in which all ModuleTemplates should be applied.
	Namespace       string
	SSAPatchOptions *client.PatchOptions
}

type RemoteCatalog struct {
	kcpClient                         client.Client
	skrContextFactory                 SkrContextProvider
	settings                          Settings
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
type moduleTemplateSyncAPIFactory func(kcpClient, skrClient client.Client, settings *Settings) moduleTemplateSyncAPI

// moduleReleaseMetaSyncAPIFactory is a function that creates moduleReleaseMetaSyncAPI instances.
type moduleReleaseMetaSyncAPIFactory func(kcpClient, skrClient client.Client, settings *Settings) moduleReleaseMetaSyncAPI

func NewRemoteCatalogFromKyma(kcpClient client.Client, skrContextFactory SkrContextProvider,
	remoteSyncNamespace string,
) *RemoteCatalog {
	force := true
	return newRemoteCatalog(kcpClient, skrContextFactory,
		Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: moduleCatalogSyncFieldManager, Force: &force},
			Namespace:       remoteSyncNamespace,
		},
	)
}

func newRemoteCatalog(kcpClient client.Client, skrContextFactory SkrContextProvider, settings Settings) *RemoteCatalog {
	var moduleTemplateSyncerAPIFactoryFn moduleTemplateSyncAPIFactory = func(kcpClient, skrClient client.Client, settings *Settings) moduleTemplateSyncAPI {
		return newModuleTemplateSyncer(kcpClient, skrClient, settings)
	}

	var moduleReleaseMetaSyncerAPIFactoryFn moduleReleaseMetaSyncAPIFactory = func(kcpClient, skrClient client.Client, settings *Settings) moduleReleaseMetaSyncAPI {
		return newModuleReleaseMetaSyncer(kcpClient, skrClient, settings)
	}

	res := &RemoteCatalog{
		kcpClient:                         kcpClient,
		skrContextFactory:                 skrContextFactory,
		settings:                          settings,
		moduleTemplateSyncAPIFactoryFn:    moduleTemplateSyncerAPIFactoryFn,
		moduleReleaseMetaSyncAPIFactoryFn: moduleReleaseMetaSyncerAPIFactoryFn,
	}

	return res
}

func (c *RemoteCatalog) SyncModuleCatalog(ctx context.Context, kyma *v1beta2.Kyma) error {
	moduleReleaseMetas, err := c.GetModuleReleaseMetasToSync(ctx, kyma)
	if err != nil {
		return err
	}

	moduleTemplates, err := c.GetModuleTemplatesToSync(ctx, &moduleReleaseMetas)
	if err != nil {
		return err
	}

	// TODO: https://github.com/kyma-project/lifecycle-manager/issues/2096
	// Remove this block after the migration to the new ModuleTemplate format is completed.
	oldModuleTemplate, err := c.GetOldModuleTemplatesToSync(ctx, kyma)
	if err != nil {
		return err
	}
	moduleTemplates = append(moduleTemplates, oldModuleTemplate...)

	return c.sync(ctx, kyma.GetNamespacedName(), moduleTemplates, moduleReleaseMetas)
}

func (c *RemoteCatalog) sync(
	ctx context.Context,
	kyma types.NamespacedName,
	kcpModules []v1beta2.ModuleTemplate,
	kcpModuleReleaseMeta []v1beta2.ModuleReleaseMeta,
) error {
	skrContext, err := c.skrContextFactory.Get(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SKR context: %w", err)
	}

	moduleTemplates := c.moduleTemplateSyncAPIFactoryFn(c.kcpClient, skrContext.Client, &c.settings)
	moduleReleaseMetas := c.moduleReleaseMetaSyncAPIFactoryFn(c.kcpClient, skrContext.Client, &c.settings)

	mtErr := moduleTemplates.SyncToSKR(ctx, kcpModules)
	mrmErr := moduleReleaseMetas.SyncToSKR(ctx, kcpModuleReleaseMeta)

	return errors.Join(mtErr, mrmErr)
}

func (c *RemoteCatalog) Delete(
	ctx context.Context,
	kyma types.NamespacedName,
) error {
	skrContext, err := c.skrContextFactory.Get(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SKR context: %w", err)
	}

	moduleTemplates := c.moduleTemplateSyncAPIFactoryFn(c.kcpClient, skrContext.Client, &c.settings)
	return moduleTemplates.DeleteAllManaged(ctx)
}

// GetModuleReleaseMetasToSync returns a list of ModuleReleaseMetas that should be synced to the SKR.
// A ModuleReleaseMeta that is Beta or Internal is synced only if the Kyma is also Beta or Internal.
func (c *RemoteCatalog) GetModuleReleaseMetasToSync(
	ctx context.Context,
	kyma *v1beta2.Kyma,
) ([]v1beta2.ModuleReleaseMeta, error) {
	moduleReleaseMetaList := &v1beta2.ModuleReleaseMetaList{}
	if err := c.kcpClient.List(ctx, moduleReleaseMetaList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ModuleReleaseMetas: %w", err)
	}

	filteredModuleReleaseMetas := []v1beta2.ModuleReleaseMeta{}
	for _, moduleReleaseMeta := range moduleReleaseMetaList.Items {
		if moduleReleaseMeta.IsBeta() && !kyma.IsBeta() {
			continue
		}
		if moduleReleaseMeta.IsInternal() && !kyma.IsInternal() {
			continue
		}
		filteredModuleReleaseMetas = append(filteredModuleReleaseMetas, moduleReleaseMeta)
	}

	return filteredModuleReleaseMetas, nil
}

// GetModuleTemplatesToSync returns a list of ModuleTemplates that should be synced to the SKR.
// A ModuleTemplate is synced if it is not mandatory and does not have sync disabled. In addition,
// it must be referenced by a ModuleReleaseMeta that is synced.
func (c *RemoteCatalog) GetModuleTemplatesToSync(
	ctx context.Context,
	moduleReleaseMetas *[]v1beta2.ModuleReleaseMeta,
) ([]v1beta2.ModuleTemplate, error) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := c.kcpClient.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("failed to list ModuleTemplates: %w", err)
	}

	moduleTemplatesToSync := map[string]bool{}
	for _, moduleReleaseMeta := range *moduleReleaseMetas {
		for _, channel := range moduleReleaseMeta.Spec.Channels {
			moduleTemplatesToSync[fmt.Sprintf("%s-%s", moduleReleaseMeta.Spec.ModuleName, channel.Version)] = true
		}
	}

	moduleTemplates := []v1beta2.ModuleTemplate{}
	for _, moduleTemplate := range moduleTemplateList.Items {
		if moduleTemplate.IsMandatory() {
			continue
		}

		if moduleTemplate.HasSyncDisabled() {
			continue
		}

		if _, found := moduleTemplatesToSync[moduleTemplate.Name]; found {
			moduleTemplates = append(moduleTemplates, moduleTemplate)
		}
	}

	return moduleTemplates, nil
}

// TODO: https://github.com/kyma-project/lifecycle-manager/issues/2096
// Remove this function after the migration to the new ModuleTemplate format is completed.
func (c *RemoteCatalog) GetOldModuleTemplatesToSync(
	ctx context.Context,
	kyma *v1beta2.Kyma,
) ([]v1beta2.ModuleTemplate, error) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := c.kcpClient.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
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
