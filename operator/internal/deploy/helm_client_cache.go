package deploy

import (
	"sync"

	moduletypes "github.com/kyma-project/module-manager/operator/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SkrChartClientCache struct {
	cache *sync.Map
}


func NewSKRChartClientCache() *SkrChartClientCache {
	return &SkrChartClientCache{
		cache: &sync.Map{},
	}
}

func (sc *SkrChartClientCache) Get(key client.ObjectKey) moduletypes.HelmClient {
	value, ok := sc.cache.Load(key)
	if !ok {
		return nil
	}
	return value.(moduletypes.HelmClient)
}

func (sc *SkrChartClientCache) Set(key client.ObjectKey, helmClient moduletypes.HelmClient) {
	sc.cache.Store(key, helmClient)
}

func (sc *SkrChartClientCache) Delete(key client.ObjectKey) {
	sc.cache.Delete(key)
}
