package skrsync

import (
	"context"

	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

// SkrCrdApplier upserts the given CRD on the SKR cluster of a Kyma using Server-Side Apply.
type SkrCrdApplier interface {
	Apply(ctx context.Context, kymaName types.NamespacedName, kcpCrd *apiextensionsv1.CustomResourceDefinition) error
}

// KcpCrdReader reads CustomResourceDefinitions from KCP by their fully qualified name.
type KcpCrdReader interface {
	Get(ctx context.Context, name string) (*apiextensionsv1.CustomResourceDefinition, error)
}

// SecretRepository reads a Secret by name from the control plane.
type SecretRepository interface {
	Get(ctx context.Context, name string) (*apicorev1.Secret, error)
}

// Service is the facade for all SKR-bound synchronization concerns of a Kyma.
// Individual concerns are implemented as private collaborators and reached only through this facade.
type Service struct {
	crdSync             *crdSync
	imagePullSecretSync *imagePullSecretSync
}

// NewService wires the SKR sync service with the dependencies for CRD and image pull secret synchronization.
// crdEntries lists the CRDs to sync in the order they should be applied.
func NewService(
	kcpCrdReader KcpCrdReader,
	crdEntries []SkrCrdSyncEntry,
	skrContextFactory remote.SkrContextProvider,
	secretRepository SecretRepository,
	imagePullSecretName string,
) *Service {
	return &Service{
		crdSync:             newCrdSync(kcpCrdReader, crdEntries),
		imagePullSecretSync: newImagePullSecretSync(secretRepository, skrContextFactory, imagePullSecretName),
	}
}

// SyncCRDs upserts the KLM-managed CRDs on the SKR cluster. It always issues SSA patches
// and leaves it to the SKR API server to decide whether a write is necessary.
func (s *Service) SyncCRDs(ctx context.Context, kyma *v1beta2.Kyma) error {
	return s.crdSync.execute(ctx, kyma)
}

// SyncImagePullSecret copies the configured image pull secret from KCP to the SKR cluster of the given Kyma.
// Returns ErrImagePullSecretNotConfigured when no secret name is configured on the service.
func (s *Service) SyncImagePullSecret(ctx context.Context, kyma types.NamespacedName) error {
	return s.imagePullSecretSync.execute(ctx, kyma)
}
