package internal

import (
	"sync"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CustomResourceDefinitionCache struct {
	storage *sync.Map
}

func NewCustomResourceDefinitionCache() *CustomResourceDefinitionCache {
	return &CustomResourceDefinitionCache{storage: &sync.Map{}}
}

func (cache *CustomResourceDefinitionCache) Get(key client.ObjectKey) *v1.CustomResourceDefinition {
	value, ok := cache.storage.Load(key)
	if !ok {
		return nil
	}

	return value.(*v1.CustomResourceDefinition)
}

func (cache *CustomResourceDefinitionCache) Set(key client.ObjectKey, value *v1.CustomResourceDefinition) {
	cache.storage.Store(key, value)
}
