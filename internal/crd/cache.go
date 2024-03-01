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
}

func NewCache(internalCache *sync.Map, metrics Metrics) *Cache {
	if internalCache == nil {
		return &Cache{
			cache:   &sync.Map{},
			metrics: metrics,
		}
	}

	return &Cache{
		cache:   internalCache,
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
	c.cache.Store(key, value)
	if c.metrics == nil {
		return
	}
	length := 0
	c.cache.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	c.metrics.UpdateCrdTotal(length)
}
