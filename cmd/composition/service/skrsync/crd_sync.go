package skrsync

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/repository/crd"
	skrcrdrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/crd"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrsync"
)

// ComposeCrdSyncService wires the CRD sync service for the KLM-managed CRDs.
// It uses the KCP client to read the desired CRDs from KCP and the SKR client cache to apply
// each CRD on the SKR via Server-Side Apply.
func ComposeCrdSyncService(kcpClient client.Client, skrClientCache *remote.ClientCache) *skrsync.CrdSyncService {
	skrClientRetrieverFunc := func(kymaName types.NamespacedName) (client.Client, error) {
		skrClient := skrClientCache.Get(kymaName)
		if skrClient == nil {
			return nil, fmt.Errorf("%w: Kyma %s", errorsinternal.ErrSkrClientNotFound, kymaName.String())
		}
		return skrClient, nil
	}

	kcpCrdRepo := crd.NewRepository(kcpClient)
	kymaApplier := skrcrdrepo.NewRepository(skrClientRetrieverFunc, crdName(shared.KymaKind))
	moduleTemplateApplier := skrcrdrepo.NewRepository(skrClientRetrieverFunc, crdName(shared.ModuleTemplateKind))
	moduleReleaseMetaApplier := skrcrdrepo.NewRepository(skrClientRetrieverFunc, crdName(shared.ModuleReleaseMetaKind))

	return skrsync.NewCrdSyncService(kcpCrdRepo, kymaApplier, moduleTemplateApplier, moduleReleaseMetaApplier)
}

func crdName(kind shared.Kind) string {
	return fmt.Sprintf("%s.%s", kind.Plural(), shared.OperatorGroup)
}
