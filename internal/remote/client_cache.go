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

// TODO: I think the only reason for using a custom remote.Client interface here is that we want to use the config
// However, the only use for the config is in SkrContextProvider where we re-use the same QPS and burst for the SKR
// Client. I don't think this makes much sense. We should consider to change this to just client.Client to make things
// re-usable easier.
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

func (c *ClientCache) Contains(key client.ObjectKey) bool {
	return c.internal.Has(key)
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
