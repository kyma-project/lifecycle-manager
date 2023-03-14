package v2

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Checking compliance with the interface methods implemented below.
var _ client.Client = &SingletonClients{}

// ProxyClient holds information required to proxy Client requests to verify RESTMapper integrity.
// During the proxy, the underlying mapper verifies mapping for the calling resource.
// If not available and NoMatchesKind error occurs, the mappings are reset (if type meta.ResettableRESTMapper).
// After reset a follow-up call verifies if mappings are now available.
type ProxyClient struct {
	config     *rest.Config
	mapper     meta.RESTMapper
	baseClient client.Client
}

// NewClientProxy returns a new instance of ProxyClient.
func NewClientProxy(config *rest.Config, mapper meta.RESTMapper) (client.Client, error) {
	baseClient, err := client.New(config, client.Options{Mapper: mapper})
	if err != nil {
		return nil, err
	}

	return &ProxyClient{
		config:     config,
		mapper:     mapper,
		baseClient: baseClient,
	}, nil
}

// Scheme returns the scheme this client is using.
func (p *ProxyClient) Scheme() *runtime.Scheme {
	return p.baseClient.Scheme()
}

// RESTMapper returns the rest mapper this client is using.
func (p *ProxyClient) RESTMapper() meta.RESTMapper {
	return p.baseClient.RESTMapper()
}

// Create implements client.Client.
func (p *ProxyClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, err := getResourceMapping(obj, p.mapper, true); err != nil {
		return err
	}
	return p.baseClient.Create(ctx, obj, opts...)
}

// Update implements client.Client.
func (p *ProxyClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, err := getResourceMapping(obj, p.mapper, true); err != nil {
		return err
	}
	return p.baseClient.Update(ctx, obj, opts...)
}

// Delete implements client.Client.
func (p *ProxyClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, err := getResourceMapping(obj, p.mapper, true); err != nil {
		return err
	}
	return p.baseClient.Delete(ctx, obj, opts...)
}

// DeleteAllOf implements client.Client.
func (p *ProxyClient) DeleteAllOf(
	ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption,
) error {
	if _, err := getResourceMapping(obj, p.mapper, true); err != nil {
		return err
	}
	return p.baseClient.DeleteAllOf(ctx, obj, opts...)
}

// Patch implements client.Client.
func (p *ProxyClient) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption,
) error {
	if _, err := getResourceMapping(obj, p.mapper, true); err != nil {
		return err
	}
	return p.baseClient.Patch(ctx, obj, patch, opts...)
}

// Get implements client.Client.
func (p *ProxyClient) Get(ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	if _, err := getResourceMapping(obj, p.mapper, true); err != nil {
		return err
	}
	return p.baseClient.Get(ctx, key, obj, opts...)
}

// List implements client.Client.
func (p *ProxyClient) List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	if _, err := getResourceMapping(obj, p.mapper, true); err != nil {
		return err
	}
	return p.baseClient.List(ctx, obj, opts...)
}

// Status implements client.StatusClient.
func (p *ProxyClient) Status() client.StatusWriter {
	return p.baseClient.Status()
}

func (p *ProxyClient) SubResource(subResource string) client.SubResourceClient {
	return p.baseClient.SubResource(subResource)
}
