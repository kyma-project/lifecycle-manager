package remote

import (
	"math/rand"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ttl              = 24 * time.Hour
	jitterMaxSeconds = 120
)

func NewClientCache() *ClientCache {
	cache := &ClientCache{internal: *ttlcache.New[client.ObjectKey, Client]()}
	go cache.internal.Start()
	return cache
}

type ClientCache struct {
	internal ttlcache.Cache[client.ObjectKey, Client]
}

func (c *ClientCache) Get(key client.ObjectKey) Client {
	ok := c.internal.Has(key)
	if !ok {
		return nil
	}
	value := c.internal.Get(key).Value()
	clnt, ok := value.(Client)
	if !ok {
		return nil
	}

	return clnt
}

func (c *ClientCache) Add(key client.ObjectKey, value Client) {
	c.internal.Set(key, value, ttl+jitter())
}

func (c *ClientCache) Delete(key client.ObjectKey) {
	c.internal.Delete(key)
}

func jitter() time.Duration {
	return time.Duration(rand.Intn(jitterMaxSeconds)) * time.Second
}
