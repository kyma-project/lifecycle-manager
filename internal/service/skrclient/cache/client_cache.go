package cache

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/jellydator/ttlcache/v3"

	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

const (
	// TTL is between 23 and 25 hours.
	ttlInSecondsLower, ttlInSecondsUpper = 23 * 60 * 60, 25 * 60 * 60
)

type Service struct {
	internal *ttlcache.Cache[string, *skrclient.SKRClient]
}

func NewService() *Service {
	cache := &Service{internal: ttlcache.New[string, *skrclient.SKRClient]()}
	go cache.internal.Start()
	return cache
}

func (m *Service) GetClient(key string) *skrclient.SKRClient {
	cachedClient := m.internal.Get(key)
	if cachedClient != nil {
		return cachedClient.Value()
	}
	return nil
}

func (m *Service) AddClient(key string, value *skrclient.SKRClient) {
	m.internal.Set(key, value, getRandomTTL())
}

func (m *Service) DeleteClient(key string) {
	m.internal.Delete(key)
}

func (m *Service) Size() int {
	return m.internal.Len()
}

func getRandomTTL() time.Duration {
	randomRange, _ := rand.Int(rand.Reader, big.NewInt(int64(ttlInSecondsUpper-ttlInSecondsLower)))
	return time.Duration(randomRange.Int64()+int64(ttlInSecondsLower)) * time.Second
}
