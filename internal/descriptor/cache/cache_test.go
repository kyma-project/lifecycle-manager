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
	sut := cache.NewDescriptorCache()
	key := cache.DescriptorKey("key 1")

	actual := sut.Get(key)

	assert.Nil(t, actual)
}

func TestGet_ForCacheWithAnEntry_ReturnsAnEntry(t *testing.T) {
	sut := cache.NewDescriptorCache()
	key1 := cache.DescriptorKey("key 1")
	ocmDesc1 := &compdesc.ComponentDescriptor{
		ComponentSpec: compdesc.ComponentSpec{
			ObjectMeta: ocmmetav1.ObjectMeta{
				Name: "descriptor 1",
			},
		},
	}
	desc1 := &v1beta2.Descriptor{ComponentDescriptor: ocmDesc1}
	sut.Set(key1, desc1)
	assertDescriptorEqual(t, desc1, sut.Get(key1))
}

func TestGet_ForCacheWithOverwrittenEntry_ReturnsCorrectEntry(t *testing.T) {
	sut := cache.NewDescriptorCache()
	key1 := cache.DescriptorKey("key 1")
	ocmDesc1 := &compdesc.ComponentDescriptor{
		ComponentSpec: compdesc.ComponentSpec{
			ObjectMeta: ocmmetav1.ObjectMeta{
				Name: "descriptor 1",
			},
		},
	}
	desc1 := &v1beta2.Descriptor{ComponentDescriptor: ocmDesc1}
	sut.Set(key1, desc1)

	ocmDesc2 := &compdesc.ComponentDescriptor{
		ComponentSpec: compdesc.ComponentSpec{
			ObjectMeta: ocmmetav1.ObjectMeta{
				Name: "descriptor 2",
			},
		},
	}
	desc2 := &v1beta2.Descriptor{ComponentDescriptor: ocmDesc2}

	assertDescriptorNotEqual(t, desc2, sut.Get(key1))
	key2 := cache.DescriptorKey("key 2")
	assert.Nil(t, sut.Get(key2))
	sut.Set(key2, desc2)
	assertDescriptorEqual(t, desc2, sut.Get(key2))

	sut.Set(key1, desc2)
	assertDescriptorEqual(t, desc2, sut.Get(key2))
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
