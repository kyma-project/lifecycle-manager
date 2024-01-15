package v2

import (
	"sync"
)

type ClientCache interface {
	GetClientFromCache(key any) Client
	SetClientInCache(key any, client Client)
	Delete(key any)
}

type MemoryClientCache struct {
	cache sync.Map // Cluster specific
}

// NewMemorySingletonClientCache returns a new instance of MemoryClientCache.
func NewMemorySingletonClientCache() *MemoryClientCache {
	return &MemoryClientCache{
		cache: sync.Map{},
	}
}

func (r *MemoryClientCache) GetClientFromCache(key any) Client {
	value, ok := r.cache.Load(key)
	if !ok {
		return nil
	}
	clnt, ok := value.(Client)
	if !ok {
		return nil
	}
	return clnt
}

func (r *MemoryClientCache) SetClientInCache(key any, client Client) {
	r.cache.Store(key, client)
}

func (r *MemoryClientCache) Delete(key any) {
	r.cache.Delete(key)
}
