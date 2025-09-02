package cache

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"github.com/jellydator/ttlcache/v3"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

const (
	// TTL is between 23 and 25 hours.
	ttlInSecondsLower, ttlInSecondsUpper = 23 * 60 * 60, 25 * 60 * 60
)

type Service struct {
	internal *ttlcache.Cache[string, skrclient.Client]
}

func NewService() *Service {
	cache := &Service{internal: ttlcache.New[string, skrclient.Client]()}
	go cache.internal.Start()
	return cache
}

func (m *Service) GetClient(key string) skrclient.Client {
	cachedClient := m.internal.Get(key)
	if cachedClient != nil {
		return cachedClient.Value()
	}
	return nil
}

func (m *Service) AddClient(key string, value skrclient.Client) {
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

func (m *Service) GetCacheKey(manifest *v1beta2.Manifest) (string, bool) {
	kymaName, err := manifest.GetKymaName()
	if err != nil {
		return "", false
	}
	cacheKey := generateCacheKey(kymaName, manifest.GetNamespace())
	return cacheKey, true
}

func generateCacheKey(values ...string) string {
	return strings.Join(values, "|")
}
