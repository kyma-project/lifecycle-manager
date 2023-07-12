package v2

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/kyma-project/lifecycle-manager/internal"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ManifestFilePrefix = "manifest"
)

type ManifestParser interface {
	Parse(spec *Spec) (internal.ManifestResources, error)
	EvictCache(spec *Spec)
}

func NewInMemoryCachedManifestParser(ttl time.Duration) *InMemoryManifestCache {
	cache := ttlcache.New[string, internal.ManifestResources]()
	go cache.Start()
	return &InMemoryManifestCache{Cache: cache, TTL: ttl}
}

func (c *InMemoryManifestCache) EvictCache(spec *Spec) {
	c.Cache.Delete(spec.Path)
}

type InMemoryManifestCache struct {
	TTL time.Duration
	*ttlcache.Cache[string, internal.ManifestResources]
}

func (c *InMemoryManifestCache) Parse(spec *Spec,
) (internal.ManifestResources, error) {
	var err error
	item := c.Cache.Get(spec.Path)
	resources := internal.ManifestResources{}
	if item != nil {
		resources = item.Value()
	} else {
		resources, err = internal.ParseManifestToObjects(spec.Path)
		if err != nil {
			return internal.ManifestResources{}, err
		}
		c.Cache.Set(spec.Path, resources, c.TTL)
	}
	copied := &internal.ManifestResources{
		Items: make([]*unstructured.Unstructured, 0, len(resources.Items)),
	}
	for _, res := range resources.Items {
		copied.Items = append(copied.Items, res.DeepCopy())
	}
	return *copied, nil
}
