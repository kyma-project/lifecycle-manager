package filemutex_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/filemutex"
)

func TestNewMutexCache_WhenCalled_NotNil(t *testing.T) {
	cache := filemutex.NewMutexCache(nil)

	assert.NotNil(t, cache)
}

func TestNewMutexCache_WhenCalledWithInitialCache_NotNil(t *testing.T) {
	internalCache := &sync.Map{}
	cache := filemutex.NewMutexCache(internalCache)

	assert.NotNil(t, cache)
}

const key = "testKey"

func TestGetLocker_WhenCalled_CreatesMutexAndStoresInCache(t *testing.T) {
	internalCache := &sync.Map{}
	cache := filemutex.NewMutexCache(internalCache)

	locker, err := cache.GetLocker(key)

	require.NoError(t, err)
	internalStored, ok := internalCache.Load(key)
	assert.True(t, ok)
	assert.Equal(t, internalStored, locker)
}

func TestGetLocker_WhenCalledSecondTime_ReturnsStored(t *testing.T) {
	cache := filemutex.NewMutexCache(nil)

	locker, _ := cache.GetLocker(key)

	cachedLocker, err := cache.GetLocker(key)
	require.NoError(t, err)
	assert.Equal(t, locker, cachedLocker)
}

func TestGetLocker_WhenCalledWithBadKey_ReturnsErr(t *testing.T) {
	internalCache := &sync.Map{}
	cache := filemutex.NewMutexCache(internalCache)
	internalCache.Store(key, "not a mutex")

	_, err := cache.GetLocker(key)

	assert.Equal(t, filemutex.ErrMutexConversion, err)
}
