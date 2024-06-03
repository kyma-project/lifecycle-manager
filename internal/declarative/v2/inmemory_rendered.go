package v2

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/internal"
)

const (
	ManifestFilePrefix      = "manifest"
	DefaultInMemoryParseTTL = 24 * time.Hour
)

type ManifestParser interface {
	Parse(spec *Spec) (internal.ManifestResources, error)
	EvictCache(spec *Spec)
}

func NewInMemoryCachedManifestParser() *InMemoryManifestCache {
	cache := ttlcache.New[string, internal.ManifestResources]()
	go cache.Start()
	return &InMemoryManifestCache{Cache: cache, TTL: DefaultInMemoryParseTTL}
}

func (c *InMemoryManifestCache) EvictCache(spec *Spec) {
	key := generateManifestCacheKey(spec)
	c.Cache.Delete(key)
}

type InMemoryManifestCache struct {
	TTL time.Duration
	*ttlcache.Cache[string, internal.ManifestResources]
}

func (c *InMemoryManifestCache) Parse(spec *Spec,
) (internal.ManifestResources, error) {
	key := generateManifestCacheKey(spec)

	var err error
	item := c.Cache.Get(key)
	resources := internal.ManifestResources{}
	if item != nil {
		resources = item.Value()
	} else {
		resources, err = internal.ParseManifestToObjects(spec.Path)
		if err != nil {
			return internal.ManifestResources{}, fmt.Errorf("failed to parse manifest objects: %w", err)
		}
		c.Cache.Set(key, resources, c.TTL)
	}
	copied := &internal.ManifestResources{
		Items: make([]*unstructured.Unstructured, 0, len(resources.Items)),
	}
	for _, res := range resources.Items {
		copied.Items = append(copied.Items, res.DeepCopy())
	}
	return *copied, nil
}

func generateManifestCacheKey(spec *Spec) string {
	file := filepath.Join(ManifestFilePrefix, spec.Path, spec.ManifestName)
	return fmt.Sprintf("%s-%s", file, spec.Mode)
}
