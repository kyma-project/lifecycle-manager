package v2

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type ClientCache interface {
	GetClient(key string) Client
	AddClient(key string, client Client)
	DeleteClient(key string)
}

const (
	// TTL is between 23 and 25 hours.
	ttlInSecondsLower, ttlInSecondsUpper = 23 * 60 * 60, 25 * 60 * 60
)

type MemoryClientCache struct {
	internal *ttlcache.Cache[string, Client]
}

func NewMemoryClientCache() *MemoryClientCache {
	cache := &MemoryClientCache{internal: ttlcache.New[string, Client]()}
	go cache.internal.Start()
	return cache
}

func (m *MemoryClientCache) GetClient(key string) Client {
	cachedClient := m.internal.Get(key)
	if cachedClient != nil {
		return cachedClient.Value()
	}
	return nil
}

func (m *MemoryClientCache) AddClient(key string, value Client) {
	m.internal.Set(key, value, getRandomTTL())
}

func (m *MemoryClientCache) DeleteClient(key string) {
	m.internal.Delete(key)
}

func (m *MemoryClientCache) Size() int {
	return m.internal.Len()
}

func getRandomTTL() time.Duration {
	randomRange, _ := rand.Int(rand.Reader, big.NewInt(int64(ttlInSecondsUpper-ttlInSecondsLower)))
	return time.Duration(randomRange.Int64()+int64(ttlInSecondsLower)) * time.Second
}
