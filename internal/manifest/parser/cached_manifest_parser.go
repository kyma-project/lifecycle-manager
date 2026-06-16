package parser

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

const (
	// DefaultInMemoryParseTTL is the default cache TTL applied by the
	// composition root to a CachedManifestParser. Cache entries older than
	// this are re-parsed from disk.
	DefaultInMemoryParseTTL = 24 * time.Hour

	manifestFilePrefix = "manifest"
)

// CachedManifestParser parses a manifest YAML at spec.Path and caches the
// resulting resource list in memory for the configured TTL. Returned objects
// are deep-copied so callers may mutate them without affecting the cache.
type CachedManifestParser struct {
	cache *ttlcache.Cache[string, internal.ManifestResources]
	ttl   time.Duration
}

// NewCachedManifestParser starts a TTL cache and returns a parser that owns
// it. The cache evicts entries automatically once ttl expires.
func NewCachedManifestParser(ttl time.Duration) *CachedManifestParser {
	cache := ttlcache.New[string, internal.ManifestResources]()
	go cache.Start()
	return &CachedManifestParser{cache: cache, ttl: ttl}
}

// EvictCache removes the cached parse result for the given spec, forcing the
// next Parse call to re-read the manifest from disk.
func (p *CachedManifestParser) EvictCache(spec *spec.Spec) {
	p.cache.Delete(generateCacheKey(spec))
}

// Parse returns the parsed manifest resources for the given spec. The result
// is fetched from cache when available; otherwise the manifest is parsed from
// disk and the result is stored before being returned. Returned items are
// deep copies of the cached objects.
func (p *CachedManifestParser) Parse(spec *spec.Spec) (internal.ManifestResources, error) {
	key := generateCacheKey(spec)

	var err error
	item := p.cache.Get(key)
	var resources internal.ManifestResources
	if item != nil {
		resources = item.Value()
	} else {
		resources, err = internal.ParseManifestToObjects(spec.Path)
		if err != nil {
			return internal.ManifestResources{}, fmt.Errorf("failed to parse manifest objects: %w", err)
		}
		p.cache.Set(key, resources, p.ttl)
	}
	copied := &internal.ManifestResources{
		Items: make([]*unstructured.Unstructured, 0, len(resources.Items)),
	}
	for _, res := range resources.Items {
		copied.Items = append(copied.Items, res.DeepCopy())
	}
	return *copied, nil
}

func generateCacheKey(spec *spec.Spec) string {
	return filepath.Join(manifestFilePrefix, spec.Path, spec.ManifestName)
}
