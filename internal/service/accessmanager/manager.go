package accessmanager

import (
	"context"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type Service struct {
	secretRepository SecretRepository
}

var (
	ErrMoreThanOneSecretFound = errors.New("more than one secret found")
	ErrAccessSecretNotFound   = errors.New("access secret not found")
)

const kubeConfigKey = "config"

type SecretRepository interface {
	List(ctx context.Context, labelSelector k8slabels.Selector) (*apicorev1.SecretList, error)
}

func NewService(secretRepository SecretRepository) *Service {
	return &Service{
		secretRepository: secretRepository,
	}
}

func (s Service) GetAccessSecretByKyma(ctx context.Context, kymaName string) (*apicorev1.Secret, error) {
	kubeConfigSecretList, err := s.secretRepository.List(ctx,
		k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kymaName}))
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets with label %s=%s: %w", shared.KymaName, kymaName,
			errors.Join(err, ErrAccessSecretNotFound))
	}

	if len(kubeConfigSecretList.Items) < 1 {
		return nil, fmt.Errorf(
			"failed to find the secret with label %s=%s: %w",
			shared.KymaName,
			kymaName,
			ErrAccessSecretNotFound,
		)
	}

	if len(kubeConfigSecretList.Items) > 1 {
		return nil, fmt.Errorf(
			"could not safely identify the rest config source: %w", ErrMoreThanOneSecretFound)
	}
	return &kubeConfigSecretList.Items[0], nil
}

func (s Service) GetAccessRestConfigByKyma(ctx context.Context, kymaName string) (*rest.Config, error) {
	kubeConfigSecret, err := s.GetAccessSecretByKyma(ctx, kymaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get access secret by kyma: %w", err)
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigSecret.Data[kubeConfigKey])
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config from kubeconfig: %w", err)
	}
	return restConfig, nil
}
