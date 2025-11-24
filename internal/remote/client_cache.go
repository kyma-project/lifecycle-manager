package remote

import (
	"crypto/rand"
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
	internal *ttlcache.Cache[client.ObjectKey, client.Client]
}

func NewClientCache() *ClientCache {
	cache := &ClientCache{internal: ttlcache.New[client.ObjectKey, client.Client]()}
	go cache.internal.Start()
	return cache
}

func (c *ClientCache) Get(key client.ObjectKey) client.Client {
	cachedClient := c.internal.Get(key)
	if cachedClient != nil {
		return cachedClient.Value()
	}
	return nil
}

func (c *ClientCache) Add(key client.ObjectKey, value client.Client) {
	c.internal.Set(key, value, getRandomTTL())
}

func (c *ClientCache) Delete(key client.ObjectKey) {
	c.internal.Delete(key)
}

func (c *ClientCache) Size() int {
	return c.internal.Len()
}

func getRandomTTL() time.Duration {
	randomRange, _ := rand.Int(rand.Reader, big.NewInt(int64(ttlInSecondsUpper-ttlInSecondsLower)))
	return time.Duration(randomRange.Int64()+int64(ttlInSecondsLower)) * time.Second
}
