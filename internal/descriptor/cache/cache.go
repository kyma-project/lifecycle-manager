package cache

import (
	"sync"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type Metrics interface {
	UpdateDescriptorTotal(size int)
}

type DescriptorCache struct {
	cache   *sync.Map
	metrics Metrics
	size    int
}

func NewDescriptorCache(cache *sync.Map, metrics Metrics) *DescriptorCache {
	if cache == nil {
		return &DescriptorCache{
			cache:   &sync.Map{},
			metrics: metrics,
		}
	}

	return &DescriptorCache{
		cache:   cache,
		metrics: metrics,
	}
}

func (c *DescriptorCache) Get(key DescriptorKey) *v1beta2.Descriptor {
	value, ok := c.cache.Load(string(key))
	if !ok {
		return nil
	}
	desc, ok := value.(*v1beta2.Descriptor)
	if !ok {
		return nil
	}

	return &v1beta2.Descriptor{ComponentDescriptor: desc.Copy()}
}

func (c *DescriptorCache) Set(key DescriptorKey, value *v1beta2.Descriptor) {
	_, existed := c.cache.Swap(string(key), value)
	if !existed {
		c.size++
	}
	if c.metrics != nil {
		c.metrics.UpdateDescriptorTotal(c.size)
	}
}

func (c *DescriptorCache) GetSize() int {
	return c.size
}
