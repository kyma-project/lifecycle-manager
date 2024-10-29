package remote

import (
	"context"
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
	kcpClient          client.Client
	skrContextFactory  SkrContextProvider
	settings           Settings
	syncerAPIFactoryFn syncerAPIFactory
}

// syncerAPI encapsulates the top-level abstration for syncing module templates to a remote cluster.
type syncerAPI interface {
	SyncToSKR(ctx context.Context, kyma types.NamespacedName, kcpModules []v1beta2.ModuleTemplate) error
	DeleteAllManaged(ctx context.Context, kyma types.NamespacedName) error
}

// syncerAPIFactory is a function that creates a new syncerAPI.
type syncerAPIFactory func(kcpClient, skrClient client.Client, settings *Settings) syncerAPI

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
	var syncerAPIFactoryFn syncerAPIFactory = func(kcpClient, skrClient client.Client, settings *Settings) syncerAPI {
		return newSyncer(kcpClient, skrClient, settings)
	}

	res := &RemoteCatalog{
		kcpClient:          kcpClient,
		skrContextFactory:  skrContextFactory,
		settings:           settings,
		syncerAPIFactoryFn: syncerAPIFactoryFn,
	}

	return res
}

func (c *RemoteCatalog) Sync(
	ctx context.Context,
	kyma types.NamespacedName,
	kcpModules []v1beta2.ModuleTemplate,
) error {
	skrContext, err := c.skrContextFactory.Get(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SkrContext to update remote catalog: %w", err)
	}

	moduleTemplates := c.syncerAPIFactoryFn(c.kcpClient, skrContext.Client, &c.settings)
	return moduleTemplates.SyncToSKR(ctx, kyma, kcpModules)
}

func (c *RemoteCatalog) Delete(
	ctx context.Context,
	kyma types.NamespacedName,
) error {
	skrContext, err := c.skrContextFactory.Get(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SkrContext for deleting RemoteCatalog: %w", err)
	}

	moduleTemplates := c.syncerAPIFactoryFn(c.kcpClient, skrContext.Client, &c.settings)
	return moduleTemplates.DeleteAllManaged(ctx, kyma)
}
