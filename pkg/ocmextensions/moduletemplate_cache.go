package ocmextensions

import (
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"sync"
)

func NewComponentDescriptorCache() *ComponentDescriptorCache {
	return &ComponentDescriptorCache{storage: &sync.Map{}}
}

type ComponentDescriptorCache struct {
	storage *sync.Map
}

func (cache *ComponentDescriptorCache) Get(key string) *compdesc.ComponentDescriptor {
	value, ok := cache.storage.Load(key)
	if !ok {
		return nil
	}

	return value.(*compdesc.ComponentDescriptor)
}

func (cache *ComponentDescriptorCache) Set(key string, value *compdesc.ComponentDescriptor) {
	cache.storage.Store(key, value)
}
