package remote

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/event"
)

var ErrFailedToGetSkrClient = errors.New("failed to get SKR client")

type KymaSkrContextProvider struct {
	clientFactory *ClientFactory
	event         event.Event
}

func NewKymaSkrContextProvider(kcpClient client.Client,
	clientFactory *ClientFactory,
	event event.Event,
) *KymaSkrContextProvider {
	return &KymaSkrContextProvider{
		clientFactory: clientFactory,
		event:         event,
	}
}

func (k *KymaSkrContextProvider) Get(ctx context.Context, kyma types.NamespacedName) (*SkrContext, error) {
	skrClient, err := k.clientFactory.Get(ctx, kyma)
	if err != nil {
		return nil, err
	}

	if skrClient == nil {
		return nil, ErrFailedToGetSkrClient
	}

	return NewSkrContext(skrClient, k.event), nil
}
