package skrclient

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Checking compliance with the interface methods implemented below.
var _ client.Client = &SKRClient{}

// ProxyClient holds information required to proxy Client requests to verify RESTMapper integrity.
// During the proxy, the underlying mapper verifies mapping for the calling resource.
// If not available and NoMatchesKind error occurs, the mappings are reset (if type meta.ResettableRESTMapper).
// After reset a follow-up call verifies if mappings are now available.
type ProxyClient struct {
	config     *rest.Config
	mapper     meta.RESTMapper
	baseClient client.Client
}

// newClientProxy returns a new instance of ProxyClient.
func newClientProxy(config *rest.Config, mapper meta.RESTMapper) (client.Client, error) {
	baseClient, err := client.New(config, client.Options{Mapper: mapper})
	if err != nil {
		return nil, fmt.Errorf("failed to create client proxy: %w", err)
	}

	return &ProxyClient{
		config:     config,
		mapper:     mapper,
		baseClient: baseClient,
	}, nil
}

func (p *ProxyClient) GroupVersionKindFor(obj machineryruntime.Object) (schema.GroupVersionKind, error) {
	groupVersion, err := p.baseClient.GroupVersionKindFor(obj)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to fetch group version: %w", err)
	}
	return groupVersion, nil
}

func (p *ProxyClient) IsObjectNamespaced(obj machineryruntime.Object) (bool, error) {
	isNameSpaced, err := p.baseClient.IsObjectNamespaced(obj)
	if err != nil {
		return isNameSpaced, fmt.Errorf("failed to fetch group version is namespaced or not: %w", err)
	}
	return isNameSpaced, nil
}

// Scheme returns the scheme this client is using.
func (p *ProxyClient) Scheme() *machineryruntime.Scheme {
	return p.baseClient.Scheme()
}

// RESTMapper returns the rest mapper this client is using.
func (p *ProxyClient) RESTMapper() meta.RESTMapper {
	return p.baseClient.RESTMapper()
}

// Create implements client.Client.
func (p *ProxyClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, err := getResourceMapping(obj, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}
	err := p.baseClient.Create(ctx, obj, opts...)
	if err != nil {
		return fmt.Errorf("failed to create object for [%v]: %w", obj, err)
	}
	return nil
}

// Update implements client.Client.
func (p *ProxyClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, err := getResourceMapping(obj, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}
	err := p.baseClient.Update(ctx, obj, opts...)
	if err != nil {
		return fmt.Errorf("failed to update object for[%v]:%w", obj, err)
	}
	return nil
}

// Delete implements client.Client.
func (p *ProxyClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, err := getResourceMapping(obj, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}
	err := p.baseClient.Delete(ctx, obj, opts...)
	if err != nil {
		return fmt.Errorf("failed to delete object for [%v]: %w", obj, err)
	}
	return nil
}

// DeleteAllOf implements client.Client.
func (p *ProxyClient) DeleteAllOf(
	ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption,
) error {
	if _, err := getResourceMapping(obj, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}
	err := p.baseClient.DeleteAllOf(ctx, obj, opts...)
	if err != nil {
		return fmt.Errorf("failed to delete all objects for [%v]: %w", obj, err)
	}
	return nil
}

// Patch implements client.Client.
func (p *ProxyClient) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption,
) error {
	if _, err := getResourceMapping(obj, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}
	err := p.baseClient.Patch(ctx, obj, patch, opts...)
	if err != nil {
		return fmt.Errorf("failed to patch object for [%v]: %w", obj, err)
	}
	return nil
}

// Get implements client.Client.
func (p *ProxyClient) Get(ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	if _, err := getResourceMapping(obj, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}
	err := p.baseClient.Get(ctx, key, obj, opts...)
	if err != nil {
		return fmt.Errorf("failed to fetch object for [%v]: %w", obj, err)
	}
	return nil
}

// List implements client.Client.
func (p *ProxyClient) List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	if _, err := getResourceMapping(obj, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}
	err := p.baseClient.List(ctx, obj, opts...)
	if err != nil {
		return fmt.Errorf("failed to fetch object for [%v]: %w", obj, err)
	}
	return nil
}

// Status implements client.StatusClient.
func (p *ProxyClient) Status() client.StatusWriter {
	return p.baseClient.Status()
}

func (p *ProxyClient) SubResource(subResource string) client.SubResourceClient {
	return p.baseClient.SubResource(subResource)
}
