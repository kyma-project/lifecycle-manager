package v2

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/internal"
)

const ManifestFilePrefix = "manifest"

type ManifestParser interface {
	Parse(spec *Spec) (internal.ManifestResources, error)
	EvictCache(spec *Spec)
}

type InMemoryManifestCache struct {
	*ttlcache.Cache[string, internal.ManifestResources]

	TTL time.Duration
}

func NewInMemoryManifestCache(ttl time.Duration) *InMemoryManifestCache {
	cache := ttlcache.New[string, internal.ManifestResources]()
	go cache.Start()
	return &InMemoryManifestCache{Cache: cache, TTL: ttl}
}

func (c *InMemoryManifestCache) EvictCache(spec *Spec) {
	key := generateCacheKey(spec)
	c.Delete(key)
}

func (c *InMemoryManifestCache) Parse(spec *Spec,
) (internal.ManifestResources, error) {
	key := generateCacheKey(spec)

	var err error
	item := c.Get(key)
	var resources internal.ManifestResources
	if item != nil {
		resources = item.Value()
	} else {
		resources, err = internal.ParseManifestToObjects(spec.Path)
		if err != nil {
			return internal.ManifestResources{}, fmt.Errorf("failed to parse manifest objects: %w", err)
		}
		c.Set(key, resources, c.TTL)
	}
	copied := &internal.ManifestResources{
		Items: make([]*unstructured.Unstructured, 0, len(resources.Items)),
	}
	for _, res := range resources.Items {
		copied.Items = append(copied.Items, res.DeepCopy())
	}
	return *copied, nil
}

func generateCacheKey(spec *Spec) string {
	return filepath.Join(ManifestFilePrefix, spec.Path, spec.ManifestName)
}
