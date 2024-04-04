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
	ttl              = 24 * time.Hour
	jitterMaxSeconds = 120
)

type MemoryClientCache struct {
	internal ttlcache.Cache[string, Client]
}

func NewMemoryClientCache() *MemoryClientCache {
	cache := &MemoryClientCache{internal: *ttlcache.New[string, Client]()}
	go cache.internal.Start()
	return cache
}

func (m *MemoryClientCache) GetClient(key string) Client {
	ok := m.internal.Has(key)
	if !ok {
		return nil
	}
	return m.internal.Get(key).Value()
}

func (m *MemoryClientCache) AddClient(key string, value Client) {
	m.internal.Set(key, value, ttl+jitter())
}

func (m *MemoryClientCache) DeleteClient(key string) {
	m.internal.Delete(key)
}

func (m *MemoryClientCache) Size() int {
	return m.internal.Len()
}

func jitter() time.Duration {
	randJitter, _ := rand.Int(rand.Reader, big.NewInt(jitterMaxSeconds))
	return time.Duration(randJitter.Int64()+1) * time.Second
}
