package remote

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// TTL is between 23 and 25 hours.
	ttlInSecondsLower, ttlInSecondsUpper = 23 * 60 * 60, 25 * 60 * 60
)

type ClientCache struct {
	internal *ttlcache.Cache[client.ObjectKey, Client]
}

func NewClientCache(opts ...func(*ClientCache) *ClientCache) *ClientCache {
	cache := &ClientCache{
		internal: ttlcache.New(ttlcache.WithDisableTouchOnHit[client.ObjectKey, Client]()),
	}

	for _, opt := range opts {
		cache = opt(cache)
	}

	go cache.internal.Start()
	return cache
}

func (c *ClientCache) Get(key client.ObjectKey) Client {
	cachedClient := c.internal.Get(key)
	if cachedClient != nil {
		return cachedClient.Value()
	}
	return nil
}

func (c *ClientCache) Add(key client.ObjectKey, value Client) {
	c.internal.Set(key, value, getRandomTTL())
}

func (c *ClientCache) Contains(key client.ObjectKey) bool {
	return c.internal.Has(key)
}

func (c *ClientCache) Delete(key client.ObjectKey) {
	c.internal.Delete(key)
}

func (c *ClientCache) Size() int {
	return c.internal.Len()
}

func WithEvictionLogger(evictionLogger func(string)) func(*ClientCache) *ClientCache {
	return func(cCache *ClientCache) *ClientCache {
		if evictionLogger != nil {
			cacheEvictionHandler := func(ctx context.Context,
				reason ttlcache.EvictionReason,
				item *ttlcache.Item[client.ObjectKey, Client],
			) {
				evictionLogger(
					fmt.Sprintf("evicted SKRClient from cache: key=%s, reason=%d",
						item.Key().String(),
						reason,
					),
				)
			}
			cCache.internal.OnEviction(cacheEvictionHandler)
		}
		return cCache
	}
}

func getRandomTTL() time.Duration {
	randomRange, _ := rand.Int(rand.Reader, big.NewInt(int64(ttlInSecondsUpper-ttlInSecondsLower)))
	return time.Duration(randomRange.Int64()+int64(ttlInSecondsLower)) * time.Second
}
