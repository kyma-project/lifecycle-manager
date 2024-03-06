package cache_test

import (
	"testing"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
)

func TestNewDescriptorCache_WithNilCache_ReturnsNewCache(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache(nil, nil)

	assert.NotNil(t, descriptorCache)
}

func TestGet_ForCacheWithoutEntry_ReturnsNoEntry(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache(nil, nil)
	key := cache.DescriptorKey("key")

	actual := descriptorCache.Get(key)

	assert.Nil(t, actual)
}

func TestGet_ForCacheWithAnEntry_ReturnsAnEntry(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache(nil, nil)
	key := cache.DescriptorKey("key")
	ocmDesc := &compdesc.ComponentDescriptor{
		ComponentSpec: compdesc.ComponentSpec{
			ObjectMeta: ocmmetav1.ObjectMeta{
				Name: "descriptor",
			},
		},
	}
	desc := &v1beta2.Descriptor{ComponentDescriptor: ocmDesc}

	descriptorCache.Set(key, desc)

	assertDescriptorEqual(t, desc, descriptorCache.Get(key))
}

func TestGet_ForCacheWithOverwrittenEntry_ReturnsNewEntry(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache(nil, nil)
	originalKey, originalValue := cache.DescriptorKey("key 1"), &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: ocmmetav1.ObjectMeta{Name: "descriptor 1"},
			},
		},
	}
	newKey, newValue := cache.DescriptorKey("key 2"), &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: ocmmetav1.ObjectMeta{Name: "descriptor 2"},
			},
		},
	}
	descriptorCache.Set(originalKey, originalValue)
	assertDescriptorNotEqual(t, newValue, descriptorCache.Get(originalKey))
	assert.Nil(t, descriptorCache.Get(newKey))

	descriptorCache.Set(newKey, newValue)
	descriptorCache.Set(originalKey, newValue)

	assertDescriptorEqual(t, newValue, descriptorCache.Get(newKey))
	assertDescriptorEqual(t, newValue, descriptorCache.Get(originalKey))
}

func TestSet_WhenCalled_UpdatesMetrics(t *testing.T) {
	metrics := &MetricsMock{}
	descriptorCache := cache.NewDescriptorCache(nil, metrics)
	key, desc := cache.DescriptorKey("key"), &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: ocmmetav1.ObjectMeta{Name: "descriptor"},
			},
		},
	}

	descriptorCache.Set(key, desc)

	assert.True(t, metrics.Called())
}

func TestGetSize_WhenCalled_ReturnsSize(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache(nil, nil)
	key, desc := cache.DescriptorKey("key"), &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: ocmmetav1.ObjectMeta{Name: "descriptor"},
			},
		},
	}

	assert.Equal(t, 0, descriptorCache.GetSize())

	descriptorCache.Set(key, desc)

	assert.Equal(t, 1, descriptorCache.GetSize())
}

func TestSet_WhenCalledWithTheSameKey_UpdatesSize(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache(nil, nil)
	key, desc := cache.DescriptorKey("key"), &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: ocmmetav1.ObjectMeta{Name: "descriptor"},
			},
		},
	}

	descriptorCache.Set(key, desc)
	descriptorCache.Set(key, desc)

	assert.Equal(t, 1, descriptorCache.GetSize())
}

type MetricsMock struct {
	called bool
}

func (m *MetricsMock) UpdateDescriptorTotal(_ int) {
	m.called = true
}

func (m *MetricsMock) Called() bool {
	return m.called
}

func assertDescriptorEqual(t *testing.T, expected, actual *v1beta2.Descriptor) {
	t.Helper()
	if expected.Name != actual.Name {
		t.Fatalf("Expected and actual descriptors do not match: \nExpected: %#v \nActual: %#v", expected, actual)
	}
}

func assertDescriptorNotEqual(t *testing.T, expected, actual *v1beta2.Descriptor) {
	t.Helper()
	if expected.Name == actual.Name {
		t.Fatalf("Expected and actual descriptors do match: \nExpected: %#v \nActual: %#v", expected, actual)
	}
}
