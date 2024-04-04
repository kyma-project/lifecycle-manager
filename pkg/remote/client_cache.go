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

func (cache *ClientCache) Get(key client.ObjectKey) Client {
	ok := cache.internal.Has(key)
	if !ok {
		return nil
	}
	value := cache.internal.Get(key).Value()
	clnt, ok := value.(Client)
	if !ok {
		return nil
	}

	return clnt
}

func (cache *ClientCache) Add(key client.ObjectKey, value Client) {
	cache.internal.Set(key, value, ttl+jitter())
}

func (cache *ClientCache) Delete(key client.ObjectKey) {
	cache.internal.Delete(key)
}

func jitter() time.Duration {
	return time.Duration(rand.Intn(jitterMaxSeconds)) * time.Second
}
