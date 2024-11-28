package cache

import (
	"sync"
	
	_type "github.com/kyma-project/lifecycle-manager/internal/descriptor/type"
)

type DescriptorCache struct {
	cache sync.Map
}

func NewDescriptorCache() *DescriptorCache {
	return &DescriptorCache{
		cache: sync.Map{},
	}
}

func (d *DescriptorCache) Get(key DescriptorKey) *_type.Descriptor {
	value, ok := d.cache.Load(key)
	if !ok {
		return nil
	}
	desc, ok := value.(*_type.Descriptor)
	if !ok {
		return nil
	}

	return &_type.Descriptor{ComponentDescriptor: desc.Copy()}
}

func (d *DescriptorCache) Set(key DescriptorKey, value *_type.Descriptor) {
	d.cache.Store(key, value)
}
