package skrcontextimpl

import "sigs.k8s.io/controller-runtime/pkg/client"

type ClientCache struct {
	factory *DualClusterFactory
}

// ClientCache is an adapter for DualClusterFactory to provide a client cache interface.
func NewClientCache(factory *DualClusterFactory) *ClientCache {
	return &ClientCache{
		factory: factory,
	}
}

func (c *ClientCache) Get(key client.ObjectKey) client.Client {
	clnt, _ := c.factory.Get(key)
	return clnt
}
