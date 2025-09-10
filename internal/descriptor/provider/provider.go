package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

var (
	ErrDecode             = errors.New("failed to decode to descriptor target")
	ErrNilProvider        = errors.New("OCMIProvider is nil")
	ErrNilIdentity        = errors.New("component identity is nil")
	ErrNameOrVersionEmpty = errors.New("component name or version is empty")
)

type DescriptorService interface {
	GetComponentDescriptor(ctx context.Context, ocmi ocmidentity.Component) (*types.Descriptor, error)
}

type CachedDescriptorProvider struct {
	descriptorCache   *cache.DescriptorCache
	descriptorService DescriptorService
}

func NewCachedDescriptorProvider(service DescriptorService) *CachedDescriptorProvider {
	return &CachedDescriptorProvider{
		descriptorCache:   cache.NewDescriptorCache(),
		descriptorService: service,
	}
}

// Convenience interface to get the OCM identity of a component from objects
// that already have all required data.
// Then we don't have to create intermediate variables of type ocmidentity.Component.
type OCMIProvider interface {
	GetOCMIdentity() (*ocmidentity.Component, error)
}

func (c *CachedDescriptorProvider) Add(ocmi ocmidentity.Component) error {
	_, err := c.getDescriptor(ocmi, true)
	return err
}

func (c *CachedDescriptorProvider) GetDescriptor(ocmi ocmidentity.Component) (*types.Descriptor, error) {
	return c.getDescriptor(ocmi, false)
}

func (c *CachedDescriptorProvider) GetDescriptorWithIdentity(ocp OCMIProvider) (*types.Descriptor, error) {
	if ocp == nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", ErrNilProvider)
	}

	ocmi, err := ocp.GetOCMIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", err)
	}
	if ocmi == nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", ErrNilIdentity)
	}

	return c.getDescriptor(*ocmi, false)
}

func (c *CachedDescriptorProvider) getDescriptor(ocmi ocmidentity.Component, updateCache bool) (
	*types.Descriptor, error,
) {
	if ocmi.Name() == "" || ocmi.Version() == "" {
		return nil, fmt.Errorf("cannot get descriptor for component: %w", ErrNameOrVersionEmpty)
	}
	key := cache.GenerateDescriptorKey(ocmi)
	descriptor := c.descriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	descriptor, err := c.descriptorService.GetComponentDescriptor(context.Background(), ocmi)
	if err != nil {
		return nil, fmt.Errorf("error finding ComponentDescriptor: %w", err)
	}

	if updateCache {
		c.descriptorCache.Set(key, descriptor)
	}

	return descriptor, nil
}
