package filemutex

import (
	"errors"
	"sync"
)

var ErrMutexConversion = errors.New("failed to convert cached value to mutex")

type MutexCache struct {
	cache *sync.Map
}

func NewMutexCache(concurrMap *sync.Map) *MutexCache {
	if concurrMap == nil {
		return &MutexCache{cache: &sync.Map{}}
	}
	return &MutexCache{cache: concurrMap}
}

// GetLocker always returns the same sync.Locker instance for a given key.
func (m *MutexCache) GetLocker(key string) (sync.Locker, error) {
	val, ok := m.cache.Load(key)
	if !ok {
		val, _ = m.cache.LoadOrStore(key, &sync.Mutex{})
	}

	mutex, ok := val.(*sync.Mutex)
	if !ok {
		return nil, ErrMutexConversion
	}
	return mutex, nil
}
