package crd

import (
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type Cache struct {
	cache *sync.Map
}

func NewCache(internalCache *sync.Map) *Cache {
	if internalCache == nil {
		return &Cache{cache: &sync.Map{}}
	}

	return &Cache{cache: internalCache}
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
}
