package skrsync

import (
	"context"
	"errors"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

var (
	ErrImagePullSecretNotConfigured = errors.New("image pull secret not configured in service")
	ErrImagePullSecretNotFound      = errors.New("image pull secret not found")
	ErrFailedToSyncImagePullSecret  = errors.New("failed to sync image pull secret to SKR")
)

// imagePullSecretSync copies the configured image pull secret from KCP to the SKR cluster of a Kyma.
// Cluster-specific metadata is stripped before applying the secret on the SKR via Server-Side Apply.
type imagePullSecretSync struct {
	secretRepository  SecretRepository
	skrContextFactory remote.SkrContextProvider
	secretName        string
}

func newImagePullSecretSync(
	secretRepository SecretRepository,
	skrContextFactory remote.SkrContextProvider,
	secretName string,
) *imagePullSecretSync {
	return &imagePullSecretSync{
		secretRepository:  secretRepository,
		skrContextFactory: skrContextFactory,
		secretName:        secretName,
	}
}

func (s *imagePullSecretSync) execute(ctx context.Context, kyma types.NamespacedName) error {
	if s.secretName == "" {
		return ErrImagePullSecretNotConfigured
	}

	secret, err := s.secretRepository.Get(ctx, s.secretName)
	if err != nil {
		return errors.Join(ErrImagePullSecretNotFound, err)
	}

	skrContext, err := s.skrContextFactory.Get(kyma)
	if err != nil {
		return err
	}

	remoteSecret := secret.DeepCopy()
	remoteSecret.Namespace = shared.DefaultRemoteNamespace
	clearClusterSpecificMetadata(remoteSecret)

	err = skrContext.Patch(ctx, remoteSecret,
		//nolint: staticcheck // issues: #2706, #2707
		client.Apply,
		client.ForceOwnership,
		fieldowners.LegacyLifecycleManager)
	if err != nil {
		return errors.Join(ErrFailedToSyncImagePullSecret, err)
	}

	return nil
}

func clearClusterSpecificMetadata(remoteSecret *apicorev1.Secret) {
	remoteSecret.ManagedFields = nil
	remoteSecret.ResourceVersion = ""
	remoteSecret.UID = ""
	remoteSecret.CreationTimestamp = apimetav1.Time{}
	remoteSecret.Generation = 0
}
