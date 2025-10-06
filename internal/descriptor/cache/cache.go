package cache

import (
	"sync"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
)

type DescriptorCache struct {
	cache sync.Map
}

func NewDescriptorCache() *DescriptorCache {
	return &DescriptorCache{
		cache: sync.Map{},
	}
}

func (d *DescriptorCache) Get(key string) *types.Descriptor {
	value, ok := d.cache.Load(key)
	if !ok {
		return nil
	}
	desc, ok := value.(*types.Descriptor)
	if !ok {
		return nil
	}

	return &types.Descriptor{ComponentDescriptor: desc.Copy()}
}

func (d *DescriptorCache) Set(key string, value *types.Descriptor) {
	d.cache.Store(key, value)
}
