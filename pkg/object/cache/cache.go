package cache

import (
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ObjectCache struct {
	cache sync.Map
}

func NewObjectCache() *ObjectCache {
	return &ObjectCache{
		cache: sync.Map{},
	}
}

func (d *ObjectCache) Get(key ObjectKey) *unstructured.Unstructured {
	value, ok := d.cache.Load(key)
	if !ok {
		return nil
	}
	object, ok := value.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	return object
}

func (d *ObjectCache) Set(key ObjectKey, value *unstructured.Unstructured) {
	d.cache.Store(key, value)
}
