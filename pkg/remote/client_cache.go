package remote

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// TTL is between 23 and 25 hours
	ttlInSecondsLower, ttlInSecondsUpper = 23 * 60 * 60, 25 * 60 * 60
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
	return c.internal.Get(key).Value()
}

func (c *ClientCache) Add(key client.ObjectKey, value Client) {
	c.internal.Set(key, value, getRandomTtl())
}

func (c *ClientCache) Delete(key client.ObjectKey) {
	c.internal.Delete(key)
}

func (c *ClientCache) Size() int {
	return c.internal.Len()
}

func getRandomTtl() time.Duration {
	randomRange, _ := rand.Int(rand.Reader, big.NewInt(int64(ttlInSecondsUpper-ttlInSecondsLower)))
	return time.Duration(randomRange.Int64()+int64(ttlInSecondsLower)) * time.Second
}
