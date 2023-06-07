package v2

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/kyma-project/lifecycle-manager/internal"
)

const (
	ManifestFilePrefix = "manifest"
)

type ManifestParser interface {
	Parse(spec *Spec) (internal.ManifestResources, error)
}

func NewInMemoryCachedManifestParser(ttl time.Duration) *InMemoryManifestCache {
	cache := ttlcache.New[string, internal.ManifestResources]()
	go cache.Start()
	return &InMemoryManifestCache{Cache: cache, TTL: ttl}
}

type InMemoryManifestCache struct {
	TTL time.Duration
	*ttlcache.Cache[string, internal.ManifestResources]
}

func (c *InMemoryManifestCache) Parse(spec *Spec,
) (internal.ManifestResources, error) {
	file := filepath.Join(ManifestFilePrefix, spec.Path, spec.ManifestName)
	key := fmt.Sprintf("%s-%s", file, spec.Mode)

	item := c.Cache.Get(key)
	if item != nil {
		resources := item.Value()

		return resources, nil
	}

	resources, err := internal.ParseManifestToObjects(spec.Path)
	if err != nil {
		return internal.ManifestResources{}, err
	}

	c.Cache.Set(key, resources, c.TTL)

	return resources, nil
}
