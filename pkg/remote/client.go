package remote

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client interface {
	client.Client
	Config() *rest.Config
}

type ConfigAndClient struct {
	client.Client
	cfg *rest.Config
}

func (c *ConfigAndClient) Config() *rest.Config {
	return c.cfg
}

func NewClientWithConfig(clnt client.Client, cfg *rest.Config) *ConfigAndClient {
	return &ConfigAndClient{Client: clnt, cfg: cfg}
}
