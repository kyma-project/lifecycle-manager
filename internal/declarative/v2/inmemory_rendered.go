package v2

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/internal"
)

type ManifestParser interface {
	Parse(ctx context.Context, renderer Renderer, obj Object, spec *Spec) (*internal.ManifestResources, error)
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

func (c *InMemoryManifestCache) Parse(
	ctx context.Context, renderer Renderer, obj Object, spec *Spec,
) (*internal.ManifestResources, error) {
	file := filepath.Join(manifest, spec.Path, spec.ManifestName)
	hashedValues, _ := internal.CalculateHash(spec.Values)
	hash := fmt.Sprintf("%v", hashedValues)
	key := fmt.Sprintf("%s-%s-%s", file, spec.Mode, hash)

	item := c.Cache.Get(key)
	if item != nil {
		resources := item.Value()

		copied := &internal.ManifestResources{
			Items: make([]*unstructured.Unstructured, 0, len(resources.Items)),
			Blobs: resources.Blobs,
		}
		for _, res := range resources.Items {
			copied.Items = append(copied.Items, res.DeepCopy())
		}

		return copied, nil
	}

	rendered, err := renderer.Render(ctx, obj)
	if err != nil {
		return nil, err
	}

	resources, err := internal.ParseManifestStringToObjects(string(rendered))
	if err != nil {
		return nil, err
	}

	c.Cache.Set(key, *resources, c.TTL)

	copied := &internal.ManifestResources{
		Items: make([]*unstructured.Unstructured, 0, len(resources.Items)),
		Blobs: resources.Blobs,
	}
	for _, res := range resources.Items {
		copied.Items = append(copied.Items, res.DeepCopy())
	}

	return copied, nil
}
