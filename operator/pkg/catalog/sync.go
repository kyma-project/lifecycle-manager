package catalog

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
)

type Sync struct {
	client.Client
	Settings
	record.EventRecorder
	*remote.ClientCache
}

func NewSync(client client.Client, recorder record.EventRecorder, cache *remote.ClientCache, settings Settings) *Sync {
	return &Sync{Client: client, EventRecorder: recorder, ClientCache: cache, Settings: settings}
}

func (s *Sync) Run(
	ctx context.Context,
	kyma *v1alpha1.Kyma,
	moduleTemplateList *v1alpha1.ModuleTemplateList,
) error {
	if kyma.Spec.Sync.Enabled {
		if err := s.syncRemote(ctx, kyma, moduleTemplateList); err != nil {
			s.Event(kyma, "Warning", "RemoteCatalogSyncError", err.Error())
			return err
		}
		s.Event(kyma, "Normal", "CatalogSync", "catalog synced")
	}
	return nil
}

func (s *Sync) syncRemote(
	ctx context.Context,
	controlPlaneKyma *v1alpha1.Kyma,
	moduleTemplateList *v1alpha1.ModuleTemplateList,
) error {
	syncContext, err := remote.InitializeKymaSynchronizationContext(
		ctx, s.Client, controlPlaneKyma, s.ClientCache,
	)
	if err != nil {
		err = fmt.Errorf("catalog sync failed: %w", err)
		s.Event(controlPlaneKyma, "Warning", "CatalogSyncError", err.Error())
		return err
	}

	return NewRemoteCatalog(syncContext.RuntimeClient, syncContext.ControlPlaneClient, s.Settings).
		CreateOrUpdate(ctx, moduleTemplateList)
}
