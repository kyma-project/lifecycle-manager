package remote

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	restrictedmodulesvc "github.com/kyma-project/lifecycle-manager/internal/service/restrictedmodule"
)

type Settings struct {
	// this namespace flag can be used to override the namespace in which all ModuleTemplates should be applied.
	Namespace       string
	SSAPatchOptions *client.PatchOptions
}

type RemoteCatalog struct {
	kcpClient                         client.Client
	skrContextFactory                 SkrContextProvider
	settings                          Settings
	restrictedModules                 []string
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
type moduleReleaseMetaSyncAPIFactory func(kcpClient, skrClient client.Client,
	settings *Settings,
) moduleReleaseMetaSyncAPI

func NewRemoteCatalogFromKyma(kcpClient client.Client, skrContextFactory SkrContextProvider,
	remoteSyncNamespace string, restrictedModules []string,
) *RemoteCatalog {
	force := true
	return newRemoteCatalog(kcpClient, skrContextFactory,
		Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: string(fieldowners.ModuleCatalogSync), Force: &force},
			Namespace:       remoteSyncNamespace,
		},
		restrictedModules,
	)
}

func newRemoteCatalog(kcpClient client.Client, skrContextFactory SkrContextProvider,
	settings Settings, restrictedModules []string,
) *RemoteCatalog {
	var moduleTemplateSyncerAPIFactoryFn moduleTemplateSyncAPIFactory = func(kcpClient, skrClient client.Client,
		settings *Settings,
	) moduleTemplateSyncAPI {
		return newModuleTemplateSyncer(kcpClient, skrClient, settings)
	}

	var moduleReleaseMetaSyncerAPIFactoryFn moduleReleaseMetaSyncAPIFactory = func(kcpClient, skrClient client.Client,
		settings *Settings,
	) moduleReleaseMetaSyncAPI {
		return newModuleReleaseMetaSyncer(kcpClient, skrClient, settings)
	}

	res := &RemoteCatalog{
		kcpClient:                         kcpClient,
		skrContextFactory:                 skrContextFactory,
		settings:                          settings,
		restrictedModules:                 restrictedModules,
		moduleTemplateSyncAPIFactoryFn:    moduleTemplateSyncerAPIFactoryFn,
		moduleReleaseMetaSyncAPIFactoryFn: moduleReleaseMetaSyncerAPIFactoryFn,
	}

	return res
}

func (c *RemoteCatalog) SyncModuleCatalog(ctx context.Context, kyma *v1beta2.Kyma) error {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := c.kcpClient.List(ctx, moduleTemplateList); err != nil {
		return fmt.Errorf("failed to list ModuleTemplates: %w", err)
	}

	filteredModuleReleaseMetas, err := c.GetModuleReleaseMetasToSync(ctx, kyma, moduleTemplateList)
	if err != nil {
		return err
	}

	filteredModuleTemplates, err := c.GetModuleTemplatesToSync(filteredModuleReleaseMetas, kyma, moduleTemplateList)
	if err != nil {
		return err
	}

	return c.sync(ctx, kyma.GetNamespacedName(), filteredModuleTemplates, filteredModuleReleaseMetas)
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
// A ModuleReleaseMeta is synced if it has at least
// one channel-version pair whose ModuleTemplate is allowed to be synced.
// Restricted modules (in the restrictedModules list) are only synced if the MRM's kymaSelector matches the Kyma.
// Non-restricted modules with a kymaSelector are skipped.
func (c *RemoteCatalog) GetModuleReleaseMetasToSync(
	ctx context.Context,
	kyma *v1beta2.Kyma,
	moduleTemplateList *v1beta2.ModuleTemplateList,
) ([]v1beta2.ModuleReleaseMeta, error) {
	moduleReleaseMetaList := &v1beta2.ModuleReleaseMetaList{}
	if err := c.kcpClient.List(ctx, moduleReleaseMetaList); err != nil {
		return nil, fmt.Errorf("failed to list ModuleReleaseMetas: %w", err)
	}

	moduleReleaseMetas := []v1beta2.ModuleReleaseMeta{}

	for _, moduleReleaseMeta := range moduleReleaseMetaList.Items {
		if moduleReleaseMeta.Spec.Mandatory != nil {
			continue
		}

		if c.isRestrictedModule(moduleReleaseMeta.Spec.ModuleName) {
			matched, err := restrictedmodulesvc.RestrictedModuleMatch(&moduleReleaseMeta, kyma)
			if err != nil {
				logf.FromContext(ctx).Error(err, "failed to evaluate restricted module match, skipping sync",
					"moduleName", moduleReleaseMeta.Spec.ModuleName)
				continue
			}
			if !matched {
				continue
			}
		} else if moduleReleaseMeta.Spec.KymaSelector != nil {
			logf.FromContext(ctx).Info(
				"WARNING: skipping catalog sync: MRM has kymaSelector but is not a restricted module",
				"moduleName", moduleReleaseMeta.Spec.ModuleName)
			continue
		}

		allowedChannels := []v1beta2.ChannelVersionAssignment{}
		for _, channel := range moduleReleaseMeta.Spec.Channels {
			if IsAllowedModuleVersion(kyma, moduleTemplateList, moduleReleaseMeta.Spec.ModuleName, channel.Version) {
				allowedChannels = append(allowedChannels, channel)
			}
		}

		if len(allowedChannels) > 0 {
			allowedModuleReleaseMeta := moduleReleaseMeta
			allowedModuleReleaseMeta.Spec.Channels = allowedChannels
			moduleReleaseMetas = append(moduleReleaseMetas, allowedModuleReleaseMeta)
		}
	}

	return moduleReleaseMetas, nil
}

func IsAllowedModuleVersion(kyma *v1beta2.Kyma, moduleTemplateList *v1beta2.ModuleTemplateList,
	moduleName, version string,
) bool {
	for _, moduleTemplate := range moduleTemplateList.Items {
		if formatModuleName(moduleName, version) == moduleTemplate.Name {
			if moduleTemplate.SyncEnabled(kyma.IsBeta(), kyma.IsInternal()) {
				return true
			}
		}
	}

	return false
}

// GetModuleTemplatesToSync returns a list of ModuleTemplates that should be synced to the SKR.
// A ModuleTemplate is synced if it is not mandatory and does not have sync disabled, and if
// it is referenced by a ModuleReleaseMeta that is synced.
func (c *RemoteCatalog) GetModuleTemplatesToSync(
	moduleReleaseMetas []v1beta2.ModuleReleaseMeta,
	kyma *v1beta2.Kyma,
	moduleTemplateList *v1beta2.ModuleTemplateList,
) ([]v1beta2.ModuleTemplate, error) {
	return FilterAllowedModuleTemplates(moduleTemplateList.Items, moduleReleaseMetas, kyma), nil
}

func (c *RemoteCatalog) isRestrictedModule(moduleName string) bool {
	return slices.Contains(c.restrictedModules, moduleName)
}

// FilterAllowedModuleTemplates filters out ModuleTemplates that are not allowed.
// A ModuleTemplate is allowed if it is not mandatory, does not have sync disabled, and if
// it is referenced by a ModuleReleaseMeta that is synced.
func FilterAllowedModuleTemplates(
	moduleTemplates []v1beta2.ModuleTemplate,
	moduleReleaseMetas []v1beta2.ModuleReleaseMeta,
	kyma *v1beta2.Kyma,
) []v1beta2.ModuleTemplate {
	moduleTemplatesToSync := map[string]bool{}
	for _, moduleReleaseMeta := range moduleReleaseMetas {
		for _, channel := range moduleReleaseMeta.Spec.Channels {
			moduleTemplatesToSync[formatModuleName(moduleReleaseMeta.Spec.ModuleName, channel.Version)] = true
		}
	}

	filteredModuleTemplates := []v1beta2.ModuleTemplate{}
	for _, moduleTemplate := range moduleTemplates {
		if !moduleTemplate.SyncEnabled(kyma.IsBeta(), kyma.IsInternal()) {
			continue
		}

		if _, found := moduleTemplatesToSync[formatModuleName(moduleTemplate.Spec.ModuleName,
			moduleTemplate.Spec.Version)]; found {
			filteredModuleTemplates = append(filteredModuleTemplates, moduleTemplate)
		}
	}

	return filteredModuleTemplates
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

func formatModuleName(moduleName, version string) string {
	return fmt.Sprintf("%s-%s", moduleName, version)
}
