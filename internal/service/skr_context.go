package service

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

type SKRContextService struct {
	skrContextProvider *remote.KymaSkrContextProvider
}

func NewSKRContextService(client client.Client, kcpRestConfig *rest.Config, clientCache *remote.ClientCache,
	event event.Event) *SKRContextService {

	kcpClient := remote.NewClientWithConfig(client, kcpRestConfig)
	return &SKRContextService{
		skrContextProvider: remote.NewKymaSkrContextProvider(kcpClient, clientCache, event),
	}
}

func (s *SKRContextService) CacheContext(ctx context.Context, kyma types.NamespacedName) error {
	return s.skrContextProvider.Init(ctx, kyma)
}

func (s *SKRContextService) GetCache(kyma types.NamespacedName) (*remote.SkrContext, error) {
	return s.skrContextProvider.Get(kyma)
}

func (s *SKRContextService) InvalidateCache(kyma types.NamespacedName) {
	s.skrContextProvider.InvalidateCache(kyma)
}
