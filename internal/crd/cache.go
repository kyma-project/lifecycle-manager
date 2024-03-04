package crd

import (
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type Metrics interface {
	UpdateCrdTotal(size int)
}

type Cache struct {
	cache   *sync.Map
	metrics Metrics
	size    int
}

func NewCache(cache *sync.Map, metrics Metrics) *Cache {
	if cache == nil {
		return &Cache{
			cache:   &sync.Map{},
			metrics: metrics,
		}
	}

	return &Cache{
		cache:   cache,
		metrics: metrics,
	}
}

func (c *Cache) Get(key string) (apiextensionsv1.CustomResourceDefinition, bool) {
	value, ok := c.cache.Load(key)
	if !ok {
		return apiextensionsv1.CustomResourceDefinition{}, false
	}
	crd, ok := value.(apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return apiextensionsv1.CustomResourceDefinition{}, false
	}

	return crd, true
}

func (c *Cache) Add(key string, value apiextensionsv1.CustomResourceDefinition) {
	_, existed := c.cache.Swap(key, value)
	if !existed {
		c.size++
	}
	if c.metrics != nil {
		c.metrics.UpdateCrdTotal(c.size)
	}
}

func (c *Cache) GetSize() int {
	return c.size
}
