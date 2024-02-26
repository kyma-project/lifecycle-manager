package filemutex_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/filemutex"
)

func TestNewMutexCache_WhenCalled_NotNil(t *testing.T) {
	cache := filemutex.NewMutexCache(nil)

	assert.NotNil(t, cache)
}

func TestGetLocker_WhenCalled_CreatesMutexAndStoreInCache(t *testing.T) {
	internalCache := &sync.Map{}
	cache := filemutex.NewMutexCache(internalCache)
	key := "testKey"

	locker, err := cache.GetLocker(key)

	assert.Nil(t, err)
	internalStored, ok := internalCache.Load(key)
	assert.True(t, ok)
	assert.Equal(t, internalStored, locker)
}

func TestGetLocker_WhenCalledSecondTime_ReturnsStored(t *testing.T) {
	cache := filemutex.NewMutexCache(nil)
	key := "testKey"

	locker, err := cache.GetLocker(key)

	cachedLocker, err := cache.GetLocker(key)
	assert.Nil(t, err)
	assert.Equal(t, locker, cachedLocker)
}

func TestGetLocker_WhenCalledWithBadKey_ReturnsErr(t *testing.T) {
	internalCache := &sync.Map{}
	cache := filemutex.NewMutexCache(internalCache)
	key := "testKey"
	internalCache.Store(key, "not a mutex")

	_, err := cache.GetLocker(key)

	assert.Equal(t, filemutex.ErrMutexConversion, err)
}
