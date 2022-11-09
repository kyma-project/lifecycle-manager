package deploy

import (
	moduletypes "github.com/kyma-project/module-manager/operator/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

type SkrChartClientCache struct {
	sync.Map
}

//nolint:ireturn
func NewSKRChartClientCache() *SkrChartClientCache {
	return &SkrChartClientCache{
		Map: sync.Map{},
	}
}

func (c *SkrChartClientCache) Get(key client.ObjectKey) moduletypes.HelmClient {
	value, ok := c.Load(key)
	if !ok {
		return nil
	}
	return value.(moduletypes.HelmClient)
}

func (c *SkrChartClientCache) Set(key client.ObjectKey, helmClient moduletypes.HelmClient) {
	c.Store(key, helmClient)
}

func (c *SkrChartClientCache) Delete(key client.ObjectKey) {
	c.Delete(key)
}
