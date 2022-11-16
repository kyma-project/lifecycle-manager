package remote

import (
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientCacheID client.ObjectKey

func NewClientCache() *ClientCache {
	return &ClientCache{internal: &sync.Map{}}
}

// ClientCache is an optimized concurrency-safe in-memory cache based on sync.Map.
// It is mainly written so that a program that needs multiple Clients in different goroutines
// can access them without recreation. It does this by holding a concurrency-safe reference map
// based on an access key (ClientCacheID). It is not optimized for multi-write scenarios, but rather
// append-only cases where clients are expected to live longer than their calling goroutine.
//
// It thus borrows the same optimizations from it:
// The ClientCache type is optimized for when the entry for a given
// key is only ever written once but read many times, as in caches that only grow.
type ClientCache struct {
	internal *sync.Map
}

func (cache *ClientCache) Get(key ClientCacheID) client.Client {
	value, ok := cache.internal.Load(key)
	if !ok {
		return nil
	}

	return value.(client.Client)
}

func (cache *ClientCache) Set(key ClientCacheID, value client.Client) {
	cache.internal.Store(key, value)
}

func (cache *ClientCache) Del(key client.ObjectKey) {
	cache.internal.Delete(key)
}
