package skrsync

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/repository/crd"
	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	skrcrdrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/crd"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrsync"
)

// klmManagedCrdKinds are the CRD kinds that KLM ensures on every SKR cluster, in the order they get applied.
//
//nolint:gochecknoglobals // intentional package-level constant list
var klmManagedCrdKinds = []shared.Kind{
	shared.KymaKind,
	shared.ModuleTemplateKind,
	shared.ModuleReleaseMetaKind,
}

// ComposeService wires the SKR sync service for both CRD and image pull secret synchronization.
// CRDs are read from KCP via the supplied client and applied on the SKR via Server-Side Apply
// using clients resolved from skrClientCache.
func ComposeService(
	kcpClient client.Client,
	skrClientCache *remote.ClientCache,
	skrContextFactory remote.SkrContextProvider,
	kcpSecretRepository *secretrepo.Repository,
	imagePullSecretName string,
) *skrsync.Service {
	return skrsync.NewService(
		crd.NewRepository(kcpClient),
		buildCrdSyncEntries(skrClientCache),
		skrContextFactory,
		kcpSecretRepository,
		imagePullSecretName,
	)
}

func buildCrdSyncEntries(skrClientCache *remote.ClientCache) []skrsync.SkrCrdSyncEntry {
	retrieve := skrClientRetriever(skrClientCache)
	entries := make([]skrsync.SkrCrdSyncEntry, 0, len(klmManagedCrdKinds))
	for _, kind := range klmManagedCrdKinds {
		entries = append(entries, skrsync.SkrCrdSyncEntry{
			Kind:    kind,
			Applier: skrcrdrepo.NewRepository(retrieve, qualifiedCrdName(kind)),
		})
	}
	return entries
}

func skrClientRetriever(skrClientCache *remote.ClientCache) skrcrdrepo.SkrClientRetrieverFunc {
	return func(kymaName types.NamespacedName) (client.Client, error) {
		skrClient := skrClientCache.Get(kymaName)
		if skrClient == nil {
			return nil, fmt.Errorf("%w: Kyma %s", errorsinternal.ErrSkrClientNotFound, kymaName.String())
		}
		return skrClient, nil
	}
}

func qualifiedCrdName(kind shared.Kind) string {
	return fmt.Sprintf("%s.%s", kind.Plural(), shared.OperatorGroup)
}
