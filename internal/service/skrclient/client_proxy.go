package skrclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Checking compliance with the interface methods implemented below.
var _ client.Client = &SKRClient{}
var _ client.Client = &ProxyClient{}

var (
	ErrMissingAPIVersion = errors.New("apply configuration has no apiVersion")
	ErrMissingKind       = errors.New("apply configuration has no kind")
)

// applyConfigurationGVKAccessor mirrors the internal accessor interface satisfied by
// all generated apply configuration types and client.ApplyConfigurationFromUnstructured.
type applyConfigurationGVKAccessor interface {
	GetAPIVersion() *string
	GetKind() *string
}

// gvkFromApplyConfiguration extracts a GroupVersionKind from an ApplyConfiguration.
// It uses a two-tier strategy: first a cheap interface type-assertion (covers all generated
// types and ApplyConfigurationFromUnstructured), then a JSON fallback for any opaque type.
func gvkFromApplyConfiguration(obj machineryruntime.ApplyConfiguration) (schema.GroupVersionKind, error) {
	if ac, ok := obj.(applyConfigurationGVKAccessor); ok {
		apiVersionStr := ptr.Deref(ac.GetAPIVersion(), "")
		if apiVersionStr == "" {
			return schema.GroupVersionKind{}, ErrMissingAPIVersion
		}
		kindStr := ptr.Deref(ac.GetKind(), "")
		if kindStr == "" {
			return schema.GroupVersionKind{}, ErrMissingKind
		}
		gv, err := schema.ParseGroupVersion(apiVersionStr)
		if err != nil {
			return schema.GroupVersionKind{}, fmt.Errorf("failed to parse apiVersion %q: %w", apiVersionStr, err)
		}
		return gv.WithKind(kindStr), nil
	}

	// Fallback: JSON round-trip for opaque ApplyConfiguration implementations.
	raw, err := json.Marshal(obj)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to marshal apply configuration: %w", err)
	}
	var typeMeta struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &typeMeta); err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to unmarshal apply configuration type meta: %w", err)
	}
	if typeMeta.APIVersion == "" {
		return schema.GroupVersionKind{}, ErrMissingAPIVersion
	}
	if typeMeta.Kind == "" {
		return schema.GroupVersionKind{}, ErrMissingKind
	}
	gv, err := schema.ParseGroupVersion(typeMeta.APIVersion)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to parse apiVersion %q: %w", typeMeta.APIVersion, err)
	}
	return gv.WithKind(typeMeta.Kind), nil
}

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

// Apply implements client.Client.
// It extracts the GVK from the ApplyConfiguration to verify RESTMapper integrity,
// then delegates to the underlying baseClient.
func (p *ProxyClient) Apply(ctx context.Context, obj machineryruntime.ApplyConfiguration, opts ...client.ApplyOption) error {
	gvk, err := gvkFromApplyConfiguration(obj)
	if err != nil {
		return fmt.Errorf("failed to extract GVK from apply configuration: %w", err)
	}

	bearer := &unstructured.Unstructured{}
	bearer.SetGroupVersionKind(gvk)

	if _, err := getResourceMapping(bearer, p.mapper); err != nil {
		return fmt.Errorf("failed to get resource mapping: %w", err)
	}

	if err := p.baseClient.Apply(ctx, obj, opts...); err != nil {
		return fmt.Errorf("failed to apply object [%v]: %w", gvk, err)
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
