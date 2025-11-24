package remote

import (
	"context"
	"fmt"
	"net/http"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SkrAccessSecretRepo interface {
	GetAccessRestConfigByKyma(ctx context.Context, name string) (*rest.Config, error)
}

type ClientFactory struct {
	clientCache         *ClientCache
	skrAccessSecretRepo SkrAccessSecretRepo
	scheme              *machineryruntime.Scheme
	skrQps              int
	skrBurst            int
}

func NewClientFactory(clientCache *ClientCache,
	skrAccessSecretRepo SkrAccessSecretRepo,
	scheme *machineryruntime.Scheme,
	skrQps int,
	skrBurst int,
) *ClientFactory {
	return &ClientFactory{
		clientCache:         clientCache,
		skrAccessSecretRepo: skrAccessSecretRepo,
		scheme:              scheme,
		skrQps:              skrQps,
		skrBurst:            skrBurst,
	}
}

func (f *ClientFactory) Get(ctx context.Context, kyma types.NamespacedName) (client.Client, error) {
	cachedClient := f.clientCache.Get(kyma)
	if cachedClient != nil {
		return cachedClient, nil
	}

	skrClient, err := f.createSkrClientFromConfig(ctx, kyma)
	if err != nil {
		return nil, err
	}

	selfEvictingClient := NewSelfEvictingClient(skrClient, func() {
		f.clientCache.Delete(kyma)
	})

	f.clientCache.Add(kyma, selfEvictingClient)

	return selfEvictingClient, nil
}

func (f *ClientFactory) createSkrClientFromConfig(ctx context.Context, kyma types.NamespacedName) (client.Client, error) {
	restConfig, err := f.skrAccessSecretRepo.GetAccessRestConfigByKyma(ctx, kyma.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config from kubeconfig: %w", err)
	}

	// Required to prevent memory leak by avoiding caching in transport.tlsTransportCache.
	// skrClients are cached anyways.
	restConfig.Proxy = http.ProxyFromEnvironment
	restConfig.QPS = float32(f.skrQps)
	restConfig.Burst = f.skrBurst

	skrClient, err := client.New(restConfig, client.Options{Scheme: f.scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create lookup client: %w", err)
	}

	return skrClient, nil
}
