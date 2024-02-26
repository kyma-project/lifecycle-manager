package crd_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/kyma-project/lifecycle-manager/internal/crd"
)

const key = "testKey"

func TestNewCache_WhenCalled_NoCachePassedIn(t *testing.T) {
	cache := crd.NewCache(nil)

	assert.NotNil(t, cache)
}

func TestNewCache_WhenCalled_CachePassedIn(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache)

	assert.NotNil(t, cache)
}

func TestGet_WhenCalled_NotInCache(t *testing.T) {
	cache := crd.NewCache(nil)

	cachedCrd, ok := cache.Get(key)

	assert.False(t, ok)
	assert.Equal(t, v1.CustomResourceDefinition{}, cachedCrd)
}

func TestGet_WhenInCache_TypeIsWrong(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache)
	internalCache.Store(key, "not a CRD")

	cachedCrd, ok := cache.Get(key)

	assert.False(t, ok)
	assert.Equal(t, v1.CustomResourceDefinition{}, cachedCrd)
}

func TestGet_WhenInCache_TypeIsRight(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache)
	someCrd := v1.CustomResourceDefinition{}
	someCrd.Name = "some-crd"
	internalCache.Store(key, someCrd)

	cachedCrd, ok := cache.Get(key)

	assert.True(t, ok)
	assert.Equal(t, someCrd.Name, cachedCrd.Name)
	assert.Equal(t, someCrd, cachedCrd)
}

func TestAdd_WhenCalled(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache)
	someCrd := v1.CustomResourceDefinition{}
	someCrd.Name = "some-crd"

	cache.Add(key, someCrd)

	cachedValue, ok := internalCache.Load(key)
	cachedCrd := cachedValue.(v1.CustomResourceDefinition)
	assert.True(t, ok)
	assert.Equal(t, someCrd.Name, cachedCrd.Name)
	assert.Equal(t, someCrd, cachedCrd)
}
