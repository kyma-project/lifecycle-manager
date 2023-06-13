package v2

import (
	"fmt"
	"path/filepath"
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

	var err error
	item := c.Cache.Get(key)
	resources := internal.ManifestResources{}
	if item != nil {
		resources = item.Value()
	} else {
		resources, err = internal.ParseManifestToObjects(spec.Path)
		if err != nil {
			return internal.ManifestResources{}, err
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
