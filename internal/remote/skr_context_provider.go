package remote

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
)

type SkrContextProvider interface {
	Get(kyma types.NamespacedName) (*SkrContext, error)
	Init(ctx context.Context, kyma types.NamespacedName) error
	InvalidateCache(kyma types.NamespacedName)
}

type KymaSkrContextProvider struct {
	clientCache          *ClientCache
	kcpClient            Client
	event                event.Event
	accessManagerService *accessmanager.Service
}

func NewKymaSkrContextProvider(kcpClient Client,
	clientCache *ClientCache,
	event event.Event,
	accessManagerService *accessmanager.Service,
) *KymaSkrContextProvider {
	return &KymaSkrContextProvider{
		clientCache:          clientCache,
		kcpClient:            kcpClient,
		event:                event,
		accessManagerService: accessManagerService,
	}
}

var ErrSkrClientContextNotFound = errors.New("skr client context not found")

func (k *KymaSkrContextProvider) Init(ctx context.Context, kyma types.NamespacedName) error {
	if k.clientCache.Contains(kyma) {
		return nil
	}

	restConfig, err := k.accessManagerService.GetAccessRestConfigByKyma(ctx, kyma.Name)
	if err != nil {
		return fmt.Errorf("failed to create rest config from kubeconfig: %w", err)
	}

	restConfig.QPS = k.kcpClient.Config().QPS
	restConfig.Burst = k.kcpClient.Config().Burst

	// Required to prevent memory leak by avoiding caching in transport.tlsTransportCache.
	// skrClients are cached anyways.
	restConfig.Proxy = http.ProxyFromEnvironment

	clnt, err := client.New(restConfig, client.Options{Scheme: k.kcpClient.Scheme()})
	if err != nil {
		return fmt.Errorf("failed to create lookup client: %w", err)
	}

	skrClient := NewClientWithConfig(clnt, restConfig)
	k.clientCache.Add(kyma, skrClient)

	return nil
}

func (k *KymaSkrContextProvider) Get(kyma types.NamespacedName) (*SkrContext, error) {
	skrClient := k.clientCache.Get(kyma)
	if skrClient == nil {
		return nil, ErrSkrClientContextNotFound
	}

	return NewSkrContext(skrClient, k.event), nil
}

func (k *KymaSkrContextProvider) InvalidateCache(kyma types.NamespacedName) {
	k.clientCache.Delete(kyma)
}
