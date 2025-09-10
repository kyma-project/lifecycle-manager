package cache

import (
	"fmt"
	"sync"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

type DescriptorCache struct {
	cache sync.Map
}

func NewDescriptorCache() *DescriptorCache {
	return &DescriptorCache{
		cache: sync.Map{},
	}
}

func (d *DescriptorCache) Get(key DescriptorKey) *types.Descriptor {
	value, ok := d.cache.Load(key)
	if !ok {
		return nil
	}
	desc, ok := value.(*types.Descriptor)
	if !ok {
		return nil
	}

	return &types.Descriptor{ComponentDescriptor: desc.Copy()}
}

func (d *DescriptorCache) Set(key DescriptorKey, value *types.Descriptor) {
	d.cache.Store(key, value)
}

type DescriptorKey string

func GenerateDescriptorKey(ocmi ocmidentity.Component) DescriptorKey {
	return DescriptorKey(fmt.Sprintf("%s:%s", ocmi.Name(), ocmi.Version()))
}
