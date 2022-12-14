package deploy

import (
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	moduletypes "github.com/kyma-project/module-manager/pkg/types"
)

type SkrChartClientCache struct {
	cache *sync.Map
}

func NewSKRChartClientCache() *SkrChartClientCache {
	return &SkrChartClientCache{
		cache: &sync.Map{},
	}
}

func (sc *SkrChartClientCache) Get(key client.ObjectKey) moduletypes.ManifestClient {
	value, ok := sc.cache.Load(key)
	if !ok {
		return nil
	}
	return value.(moduletypes.ManifestClient)
}

func (sc *SkrChartClientCache) Set(key client.ObjectKey, helmClient moduletypes.ManifestClient) {
	sc.cache.Store(key, helmClient)
}

func (sc *SkrChartClientCache) Delete(key client.ObjectKey) {
	sc.cache.Delete(key)
}
