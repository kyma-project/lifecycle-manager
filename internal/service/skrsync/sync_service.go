package skrsync

import (
	"context"
	"errors"

	apicorev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

var (
	ErrImagePullSecretNotConfigured = errors.New("image pull secret not configured in service")
	ErrImagePullSecretNotFound      = errors.New("image pull secret not found")
	ErrFailedToSyncImagePullSecret  = errors.New("failed to sync image pull secret to SKR")
)

type SecretRepository interface {
	Get(ctx context.Context, name string) (*apicorev1.Secret, error)
}

type SyncCrdsUseCase interface {
	Execute(ctx context.Context, kyma *v1beta2.Kyma) (bool, error)
}

type Service struct {
	skrContextFactory   remote.SkrContextProvider
	secretRepository    SecretRepository
	syncCrdsUseCase     SyncCrdsUseCase
	imagePullSecretName string
}

func NewService(
	skrContextFactory remote.SkrContextProvider,
	secretRepository SecretRepository,
	syncCrdsUseCase SyncCrdsUseCase,
	imagePullSecretName string,
) *Service {
	return &Service{
		skrContextFactory:   skrContextFactory,
		secretRepository:    secretRepository,
		syncCrdsUseCase:     syncCrdsUseCase,
		imagePullSecretName: imagePullSecretName,
	}
}

func (s *Service) SyncCrds(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	return s.syncCrdsUseCase.Execute(ctx, kyma)
}

func (s *Service) SyncImagePullSecret(ctx context.Context, kyma types.NamespacedName) error {
	if s.imagePullSecretName == "" {
		return ErrImagePullSecretNotConfigured
	}

	secret, err := s.secretRepository.Get(ctx, s.imagePullSecretName)
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
		client.Apply,
		client.ForceOwnership,
		client.FieldOwner(shared.OperatorName))
	if err != nil {
		return errors.Join(ErrFailedToSyncImagePullSecret, err)
	}

	return nil
}

func clearClusterSpecificMetadata(remoteSecret *apicorev1.Secret) {
	remoteSecret.ManagedFields = nil
	remoteSecret.ResourceVersion = ""
	remoteSecret.UID = ""
	remoteSecret.CreationTimestamp = v1.Time{}
	remoteSecret.Generation = 0
}
