package crd_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/internal/crd"
)

const key = "testKey"

func TestNewCache_WhenCalled_NoCachePassedIn(t *testing.T) {
	cache := crd.NewCache(nil, nil)

	assert.NotNil(t, cache)
}

func TestNewCache_WhenCalled_CachePassedIn(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache, nil)

	assert.NotNil(t, cache)
}

func TestGet_WhenCalled_NotInCache(t *testing.T) {
	cache := crd.NewCache(nil, nil)

	cachedCrd, ok := cache.Get(key)

	assert.False(t, ok)
	assert.Equal(t, apiextensionsv1.CustomResourceDefinition{}, cachedCrd)
}

func TestGet_WhenInCache_TypeIsWrong(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache, nil)
	internalCache.Store(key, "not a CRD")

	cachedCrd, ok := cache.Get(key)

	assert.False(t, ok)
	assert.Equal(t, apiextensionsv1.CustomResourceDefinition{}, cachedCrd)
}

const crdName = "some-crd"

func TestGet_WhenInCache_TypeIsRight(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache, nil)
	someCrd := apiextensionsv1.CustomResourceDefinition{ObjectMeta: apimetav1.ObjectMeta{Name: crdName}}
	internalCache.Store(key, someCrd)

	cachedCrd, ok := cache.Get(key)

	assert.True(t, ok)
	assert.Equal(t, someCrd, cachedCrd)
}

func TestAdd_WhenCalled(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache, nil)
	someCrd := apiextensionsv1.CustomResourceDefinition{ObjectMeta: apimetav1.ObjectMeta{Name: crdName}}

	cache.Add(key, someCrd)

	cachedValue, ok := internalCache.Load(key)
	assert.True(t, ok)
	cachedCrd, ok := cachedValue.(apiextensionsv1.CustomResourceDefinition)
	assert.True(t, ok)
	assert.Equal(t, someCrd, cachedCrd)
}

func TestAdd_WhenCalled_AlreadyExists(t *testing.T) {
	internalCache := &sync.Map{}
	cache := crd.NewCache(internalCache, nil)
	someCrd := apiextensionsv1.CustomResourceDefinition{ObjectMeta: apimetav1.ObjectMeta{Name: crdName}}
	internalCache.Store(key, someCrd)

	newCrd := apiextensionsv1.CustomResourceDefinition{}
	newCrd.Name = "new-crd"

	cache.Add(key, newCrd)

	cachedValue, ok := internalCache.Load(key)
	assert.True(t, ok)
	cachedCrd, ok := cachedValue.(apiextensionsv1.CustomResourceDefinition)
	assert.True(t, ok)
	assert.Equal(t, newCrd, cachedCrd)
}

func TestAdd_WhenCalled_UpdatesMetrics(t *testing.T) {
	metrics := &MetricsMock{}
	cache := crd.NewCache(nil, metrics)
	someCrd := apiextensionsv1.CustomResourceDefinition{ObjectMeta: apimetav1.ObjectMeta{Name: crdName}}

	cache.Add(key, someCrd)

	assert.True(t, metrics.Called())
}

func TestGetSize_WhenCalled_ReturnsSize(t *testing.T) {
	cache := crd.NewCache(nil, nil)
	someCrd := apiextensionsv1.CustomResourceDefinition{ObjectMeta: apimetav1.ObjectMeta{Name: crdName}}
	cache.Add(key, someCrd)

	assert.Equal(t, 1, cache.GetSize())
}

func TestAdd_WhenCalledWithTheSameKey_UpdatesSize(t *testing.T) {
	cache := crd.NewCache(nil, nil)
	someCrd := apiextensionsv1.CustomResourceDefinition{ObjectMeta: apimetav1.ObjectMeta{Name: crdName}}

	cache.Add(key, someCrd)
	cache.Add(key, someCrd)

	assert.Equal(t, 1, cache.GetSize())
}

type MetricsMock struct {
	called bool
}

func (m *MetricsMock) UpdateCrdTotal(_ int) {
	m.called = true
}

func (m *MetricsMock) Called() bool {
	return m.called
}
