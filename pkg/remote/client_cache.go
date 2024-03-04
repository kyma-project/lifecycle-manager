package remote

import (
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Metrics interface {
	UpdateClientTotal(size int)
}

func NewClientCache(cache *sync.Map, metrics Metrics) *ClientCache {
	if cache == nil {
		return &ClientCache{
			cache:   &sync.Map{},
			metrics: metrics,
		}
	}
	return &ClientCache{
		cache:   cache,
		metrics: metrics,
	}
}

type ClientCache struct {
	cache   *sync.Map
	metrics Metrics
	size    int
}

func (c *ClientCache) Get(key client.ObjectKey) Client {
	value, ok := c.cache.Load(key)
	if !ok {
		return nil
	}
	clnt, ok := value.(Client)
	if !ok {
		return nil
	}

	return clnt
}

func (c *ClientCache) Set(key client.ObjectKey, value Client) {
	_, existed := c.cache.Swap(key, value)
	if !existed {
		c.size++
	}
	if c.metrics != nil {
		c.metrics.UpdateClientTotal(c.size)
	}
}

func (c *ClientCache) Del(key client.ObjectKey) {
	_, existed := c.cache.LoadAndDelete(key)
	if !existed {
		c.size--
	}
	if c.metrics != nil {
		c.metrics.UpdateClientTotal(c.size)
	}
}

func (c *ClientCache) GetSize() int {
	return c.size
}
