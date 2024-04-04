package v2

import (
	"math/rand"
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
	value := m.internal.Get(key).Value()
	clnt, ok := value.(Client)
	if !ok {
		return nil
	}

	return clnt
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
	return time.Duration(rand.Intn(jitterMaxSeconds)) * time.Second
}
