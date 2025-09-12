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
	ErrTypeAssert    = errors.New("failed to convert to v1beta2.Descriptor")
	ErrDecode        = errors.New("failed to decode to descriptor target")
	ErrTemplateNil   = errors.New("module template is nil")
	ErrDescriptorNil = errors.New("module template contains nil descriptor")
	ErrNoIdentity    = errors.New("component identity is nil")
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

type OCMIProvider interface {
	GetOCMI() (*ocmidentity.ComponentIdentity, error)
}

// [Review note] I am leaving that function as only this one adds new items to the cache.
func (c *CachedDescriptorProvider) Add(ocmi ocmidentity.ComponentIdentity) error {
	_, err := c.getDescriptor(ocmi, true)
	return err
}

// [Review note] Signature of this function has changed because:
//  1. We no longer want to read the descriptor from the ModuleTemplate instance.
//  2. After we remove ModuleTemplate.Spec.Descriptor attribute, the remaining attributes of the
//     ModuleTemplate doesn't provide *enough* information to uniquely identify a
//     Component: the full OCM Component Name is missing.
func (c *CachedDescriptorProvider) GetDescriptor(ocmi ocmidentity.ComponentIdentity) (*types.Descriptor, error) {
	return c.getDescriptor(ocmi, false)
}

func (c *CachedDescriptorProvider) GetDescriptorWithIdentity(ocp OCMIProvider) (*types.Descriptor, error) {
	ocmi, err := ocp.GetOCMI()
	if err != nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", err)
	}
	if ocmi == nil {
		return nil, fmt.Errorf("failed to get component identity from provider: %w", ErrNoIdentity)
	}

	return c.getDescriptor(*ocmi, false)
}

func (c *CachedDescriptorProvider) getDescriptor(ocmi ocmidentity.ComponentIdentity, updateCache bool) (*types.Descriptor, error) {
	key := cache.GenerateDescriptorKey(ocmi.ComponentName, ocmi.ComponentVersion)
	descriptor := c.DescriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	descriptor, err := c.CompDescService.GetComponentDescriptor(ocmi.ComponentName, ocmi.ComponentVersion)
	if err != nil {
		return nil, fmt.Errorf("error finding ComponentDescriptor: %w", err)
	}

	if updateCache {
		c.DescriptorCache.Set(key, descriptor)
	}

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
