package provider

import (
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/service/componentdescriptor"
)

var (
	ErrTypeAssert    = errors.New("failed to convert to v1beta2.Descriptor")
	ErrDecode        = errors.New("failed to decode to descriptor target")
	ErrTemplateNil   = errors.New("module template is nil")
	ErrDescriptorNil = errors.New("module template contains nil descriptor")
)

type CachedDescriptorProvider struct {
	DescriptorCache *cache.DescriptorCache
	CompDescService componentdescriptor.Service
}

func NewCachedDescriptorProvider() *CachedDescriptorProvider {

	return &CachedDescriptorProvider{
		DescriptorCache: cache.NewDescriptorCache(),
	}
}

// OCMComponentIdentity uniquely identifies an OCM Component.
// See: https://ocm.software/docs/overview/important-terms/#component-identity
type OCMComponentIdentity struct {
	ComponentName    string
	ComponentVersion string
}

func (c *CachedDescriptorProvider) Add(ocmi OCMComponentIdentity) error {
	key := cache.GenerateDescriptorKey(ocmi.ComponentName, ocmi.ComponentVersion)
	descriptor := c.DescriptorCache.Get(key)
	if descriptor != nil {
		return nil
	}

	//TODO: Implement!
	//Fetch the descriptor from the OCI repository
	//c.DescriptorCache.Set(key, descriptor)
	return nil
}

// [Review note] Signature of this function has changed because:
//  1. We no longer want to read the descriptor from the ModuleTemplate instance.
//  2. After we remove ModuleTemplate.Spec.Descriptor attribute, the remaining attributes of the
//     ModuleTemplate doesn't provide *enough* information to uniquely identify a
//     Component: the full OCM Component Name is missing.
func (c *CachedDescriptorProvider) GetDescriptor(ocmi OCMComponentIdentity) (*types.Descriptor, error) {
	key := cache.GenerateDescriptorKey(ocmi.ComponentName, ocmi.ComponentVersion)
	descriptor := c.DescriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	descriptor, err := c.CompDescService.GetComponentDescriptor(ocmi.ComponentName, ocmi.ComponentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get ComponentDescriptor: %w", err)
	}
	c.DescriptorCache.Set(key, descriptor)
	return descriptor, nil
}

/*
func (c *CachedDescriptorProvider) getDescriptorOld(template *v1beta2.ModuleTemplate) (*types.Descriptor, error) {
	if template == nil {
		return nil, ErrTemplateNil
	}

	if template.Spec.Descriptor.Object != nil {
		desc, ok := template.Spec.Descriptor.Object.(*types.Descriptor)
		if !ok {
			return nil, ErrTypeAssert
		}

		if desc == nil {
			return nil, ErrDescriptorNil
		}

		return desc, nil
	}
	key := cache.GenerateDescriptorKey(template)
	descriptor := c.DescriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	ocmDesc, err := compdesc.Decode(
		template.Spec.Descriptor.Raw, []compdesc.DecodeOption{compdesc.DisableValidation(true)}...,
	)
	if err != nil {
		return nil, errors.Join(ErrDecode, err)
	}

	template.Spec.Descriptor.Object = &types.Descriptor{ComponentDescriptor: ocmDesc}
	descriptor, ok := template.Spec.Descriptor.Object.(*types.Descriptor)
	if !ok {
		return nil, ErrTypeAssert
	}

	return descriptor, nil
}

func (c *CachedDescriptorProvider) addOld(template *v1beta2.ModuleTemplate) error {
	if template == nil {
		return ErrTemplateNil
	}
	key := cache.GenerateDescriptorKey(template)
	descriptor := c.DescriptorCache.Get(key)
	if descriptor != nil {
		return nil
	}

	if template.Spec.Descriptor.Object != nil {
		desc, ok := template.Spec.Descriptor.Object.(*types.Descriptor)
		if ok && desc != nil {
			c.DescriptorCache.Set(key, desc)
			return nil
		}
	}

	ocmDesc, err := compdesc.Decode(
		template.Spec.Descriptor.Raw, []compdesc.DecodeOption{compdesc.DisableValidation(true)}...,
	)
	if err != nil {
		return errors.Join(ErrDecode, err)
	}

	template.Spec.Descriptor.Object = &types.Descriptor{ComponentDescriptor: ocmDesc}
	descriptor, ok := template.Spec.Descriptor.Object.(*types.Descriptor)
	if !ok {
		return ErrTypeAssert
	}

	c.DescriptorCache.Set(key, descriptor)
	return nil
}
*/
