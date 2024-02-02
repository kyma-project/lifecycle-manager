package cache

import (
	"sync"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type DescriptorCache struct {
	cache sync.Map
}

func NewDescriptorCache() *DescriptorCache {
	return &DescriptorCache{
		cache: sync.Map{},
	}
}

func (d *DescriptorCache) Get(key DescriptorKey) *v1beta2.Descriptor {
	value, ok := d.cache.Load(key)
	if !ok {
		return nil
	}
	desc, ok := value.(*v1beta2.Descriptor)
	if !ok {
		return nil
	}

	return &v1beta2.Descriptor{ComponentDescriptor: desc.Copy()}
}

func (d *DescriptorCache) Set(key DescriptorKey, value *v1beta2.Descriptor) {
	d.cache.Store(key, value)
}
