package cache_test

import (
	"testing"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
)

func TestGet_ForCacheWithoutEntry_ReturnsNoEntry(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	key := cache.DescriptorKey("key 1")

	actual := descriptorCache.Get(key)

	assert.Nil(t, actual)
}

func TestGet_ForCacheWithAnEntry_ReturnsAnEntry(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	key1 := cache.DescriptorKey("key 1")
	ocmDesc1 := &compdesc.ComponentDescriptor{
		ComponentSpec: compdesc.ComponentSpec{
			ObjectMeta: ocmmetav1.ObjectMeta{
				Name: "descriptor 1",
			},
		},
	}
	desc1 := &v1beta2.Descriptor{ComponentDescriptor: ocmDesc1}

	descriptorCache.Set(key1, desc1)

	assertDescriptorEqual(t, desc1, descriptorCache.Get(key1))
}

func TestGet_ForCacheWithOverwrittenEntry_ReturnsNewEntry(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
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
