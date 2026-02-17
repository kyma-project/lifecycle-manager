package provider

import (
	"context"
	"errors"
	"fmt"

	descriptorcache "github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
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
	GetComponentDescriptor(ctx context.Context, ocmId ocmidentity.ComponentId) (*types.Descriptor, error)
}

type DescriptorCache interface {
	Get(key descriptorcache.DescriptorKey) *types.Descriptor
	Set(key descriptorcache.DescriptorKey, value *types.Descriptor)
}

type CachedDescriptorProvider struct {
	descriptorCache   DescriptorCache
	descriptorService DescriptorService
}

func NewCachedDescriptorProvider(service DescriptorService, descCache DescriptorCache) *CachedDescriptorProvider {
	return &CachedDescriptorProvider{
		descriptorCache:   descCache,
		descriptorService: service,
	}
}

// OCMIProvider is a convenience interface to get the OCM identity of a component from objects
// that already have all required data.
// Then we don't have to create intermediate variables of type ocmidentity.Component.
type OCMIProvider interface {
	GetOCMIdentity() (*ocmidentity.ComponentId, error)
}

func (c *CachedDescriptorProvider) Add(ocmId ocmidentity.ComponentId) error {
	if ocmId.Name() == "" || ocmId.Version() == "" {
		return fmt.Errorf("cannot get descriptor for component: %w", ErrNameOrVersionEmpty)
	}
	key := descriptorcache.GenerateDescriptorKey(ocmId)
	descriptor := c.descriptorCache.Get(key)
	if descriptor != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	descriptor, err := c.descriptorService.GetComponentDescriptor(ctx, ocmId)
	defer cancel()

	if err != nil {
		return fmt.Errorf("error finding ComponentDescriptor: %w", err)
	}

	c.descriptorCache.Set(key, descriptor)

	return nil
}

func (c *CachedDescriptorProvider) GetDescriptor(ocmId ocmidentity.ComponentId) (*types.Descriptor, error) {
	if ocmId.Name() == "" || ocmId.Version() == "" {
		return nil, fmt.Errorf("cannot get descriptor for component: %w", ErrNameOrVersionEmpty)
	}
	key := descriptorcache.GenerateDescriptorKey(ocmId)
	descriptor := c.descriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	descriptor, err := c.descriptorService.GetComponentDescriptor(ctx, ocmId)
	defer cancel()

	if err != nil {
		return nil, fmt.Errorf("error finding ComponentDescriptor: %w", err)
	}

	return descriptor, nil
}

func (c *CachedDescriptorProvider) GetDescriptorWithIdentity(ocp OCMIProvider) (*types.Descriptor, error) {
	if ocp == nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", ErrNilProvider)
	}

	ocmId, err := ocp.GetOCMIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", err)
	}
	if ocmId == nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", ErrNilIdentity)
	}

	return c.GetDescriptor(*ocmId)
}
