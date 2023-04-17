package ocmextensions

import (
	"context"
	"sync"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewComponentDescriptorCache() *ComponentDescriptorCache {
	return &ComponentDescriptorCache{storage: &sync.Map{}}
}

type ComponentDescriptorCache struct {
	storage *sync.Map
}

func (cache *ComponentDescriptorCache) get(key string) *compdesc.ComponentDescriptor {
	value, ok := cache.storage.Load(key)
	if !ok {
		return nil
	}

	return value.(*compdesc.ComponentDescriptor)
}

func (cache *ComponentDescriptorCache) set(key string, value *compdesc.ComponentDescriptor) {
	cache.storage.Store(key, value)
}

func (cache *ComponentDescriptorCache) GetRemoteDescriptor(
	ctx context.Context,
	descriptorCacheKey string,
	descriptor *v1beta1.Descriptor,
	clnt client.Client,
) (*compdesc.ComponentDescriptor, error) {
	remoteDescriptor := cache.get(descriptorCacheKey)
	if remoteDescriptor != nil {
		return remoteDescriptor, nil
	}
	remoteDescriptor, err := getRemoteDescriptor(ctx, descriptor, clnt)
	if err != nil {
		return nil, err
	}
	cache.set(descriptorCacheKey, remoteDescriptor)
	return remoteDescriptor, nil
}
