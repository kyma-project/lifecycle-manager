package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"ocm.software/ocm/api/ocm/compdesc"
	ocmmetav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"

	descriptorcache "github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
)

func TestGet_ForCacheWithoutEntry_ReturnsNoEntry(t *testing.T) {
	descriptorCache := descriptorcache.NewDescriptorCache()
	key := "key 1"

	actual := descriptorCache.Get(descriptorcache.DescriptorKey(key))

	assert.Nil(t, actual)
}

func TestGet_ForCacheWithAnEntry_ReturnsAnEntry(t *testing.T) {
	descriptorCache := descriptorcache.NewDescriptorCache()
	key1 := "key 1"
	ocmDesc1 := &compdesc.ComponentDescriptor{
		ComponentSpec: compdesc.ComponentSpec{
			ObjectMeta: ocmmetav1.ObjectMeta{
				Name: "descriptor 1",
			},
		},
	}
	desc1 := &types.Descriptor{ComponentDescriptor: ocmDesc1}

	descriptorCache.Set(descriptorcache.DescriptorKey(key1), desc1)

	assertDescriptorEqual(t, desc1, descriptorCache.Get(descriptorcache.DescriptorKey(key1)))
}

func TestGet_ForCacheWithOverwrittenEntry_ReturnsNewEntry(t *testing.T) {
	descriptorCache := descriptorcache.NewDescriptorCache()
	originalKey, originalValue := "key 1", &types.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: ocmmetav1.ObjectMeta{Name: "descriptor 1"},
			},
		},
	}
	newKey, newValue := "key 2", &types.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			ComponentSpec: compdesc.ComponentSpec{
				ObjectMeta: ocmmetav1.ObjectMeta{Name: "descriptor 2"},
			},
		},
	}
	descriptorCache.Set(descriptorcache.DescriptorKey(originalKey), originalValue)
	assertDescriptorNotEqual(t, newValue, descriptorCache.Get(descriptorcache.DescriptorKey(originalKey)))
	assert.Nil(t, descriptorCache.Get(descriptorcache.DescriptorKey(newKey)))

	descriptorCache.Set(descriptorcache.DescriptorKey(newKey), newValue)
	descriptorCache.Set(descriptorcache.DescriptorKey(originalKey), newValue)

	assertDescriptorEqual(t, newValue, descriptorCache.Get(descriptorcache.DescriptorKey(newKey)))
	assertDescriptorEqual(t, newValue, descriptorCache.Get(descriptorcache.DescriptorKey(originalKey)))
}

func assertDescriptorEqual(t *testing.T, expected, actual *types.Descriptor) {
	t.Helper()
	if expected.Name != actual.Name {
		t.Fatalf("Expected and actual descriptors do not match: \nExpected: %#v \nActual: %#v", expected, actual)
	}
}

func assertDescriptorNotEqual(t *testing.T, expected, actual *types.Descriptor) {
	t.Helper()
	if expected.Name == actual.Name {
		t.Fatalf("Expected and actual descriptors do match: \nExpected: %#v \nActual: %#v", expected, actual)
	}
}
