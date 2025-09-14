package provider

import (
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/internal/service/componentdescriptor"
)

var (
	//ErrTypeAssert    = errors.New("failed to convert to v1beta2.Descriptor")
	ErrDecode = errors.New("failed to decode to descriptor target")
	//ErrTemplateNil   = errors.New("module template is nil")
	//ErrDescriptorNil = errors.New("module template contains nil descriptor")
	ErrNoIdentity   = errors.New("component identity is nil")
	ErrNameEmpty    = errors.New("component name is empty")
	ErrVersionEmpty = errors.New("component version is empty")
)

type CachedDescriptorProvider struct {
	DescriptorCache *cache.DescriptorCache
	//TODO: Consider replacing with an interface
	CompDescService *componentdescriptor.Service
}

func NewCachedDescriptorProvider(service *componentdescriptor.Service) *CachedDescriptorProvider {
	return &CachedDescriptorProvider{
		DescriptorCache: cache.NewDescriptorCache(),
		CompDescService: service,
	}
}

// Convenience interface to get the OCM identity of a component from objects
// that already have all required data.
// Then we don't have to create intermediate variables of type ocmidentity.Component.
type OCMIProvider interface {
	GetOCMIdentity() (*ocmidentity.Component, error)
}

// [Review note] I am leaving that function as only this one adds new items to the cache.
func (c *CachedDescriptorProvider) Add(ocmi ocmidentity.Component) error {
	_, err := c.getDescriptor(ocmi, true)
	return err
}

// [Review note] Signature of this function has changed because:
//  1. We no longer want to read the descriptor from the ModuleTemplate instance.
//  2. After we remove ModuleTemplate.Spec.Descriptor attribute, the remaining attributes of the
//     ModuleTemplate doesn't provide *enough* information to uniquely identify a
//     Component: the full OCM Component Name is missing.
//
// TODO: Rename to just Get
func (c *CachedDescriptorProvider) GetDescriptor(ocmi ocmidentity.Component) (*types.Descriptor, error) {
	return c.getDescriptor(ocmi, false)
}

// TODO: Rename to GetWithIdentityProvider
func (c *CachedDescriptorProvider) GetDescriptorWithIdentity(ocp OCMIProvider) (*types.Descriptor, error) {
	ocmi, err := ocp.GetOCMIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", err)
	}
	if ocmi == nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", ErrNoIdentity)
	}

	return c.getDescriptor(*ocmi, false)
}

func (c *CachedDescriptorProvider) getDescriptor(ocmi ocmidentity.Component, updateCache bool) (*types.Descriptor, error) {
	if ocmi.Name() == "" {
		return nil, fmt.Errorf("cannot get descriptor for component: %w", ErrNameEmpty)
	}
	if ocmi.Version() == "" {
		return nil, fmt.Errorf("cannot get descriptor for component: %w", ErrVersionEmpty)
	}
	key := cache.GenerateDescriptorKey(ocmi)
	descriptor := c.DescriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	descriptor, err := c.CompDescService.GetComponentDescriptor(ocmi)
	if err != nil {
		return nil, fmt.Errorf("error finding ComponentDescriptor: %w", err)
	}

	if updateCache {
		c.DescriptorCache.Set(key, descriptor)
	}

	return descriptor, nil
}
