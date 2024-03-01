package cache

import (
	"sync"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type Metrics interface {
	UpdateDescriptorTotal(size int)
}

type DescriptorCache struct {
	cache   sync.Map
	metrics Metrics
}

func NewDescriptorCache(metrics Metrics) *DescriptorCache {
	return &DescriptorCache{
		cache:   sync.Map{},
		metrics: metrics,
	}
}

func (d *DescriptorCache) Get(key DescriptorKey) *v1beta2.Descriptor {
	value, ok := d.cache.Load(string(key))
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
	d.cache.Store(string(key), value)
	if d.metrics == nil {
		return
	}
	length := 0
	d.cache.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	d.metrics.UpdateDescriptorTotal(length)
}
