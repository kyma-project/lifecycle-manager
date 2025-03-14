package service

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/repository"
)

type KymaService struct {
	kymaRepository *repository.KymaRepository
}

func NewKymaService(client client.Client) *KymaService {
	return &KymaService{
		kymaRepository: repository.NewKymaRepository(client),
	}
}

func (s *KymaService) GetKyma(ctx context.Context, namespacedName client.ObjectKey) (*v1beta2.Kyma, error) {
	return s.kymaRepository.Get(ctx, namespacedName)
}
