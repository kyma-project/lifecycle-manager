package provider

import (
	"errors"

	"ocm.software/ocm/api/ocm/compdesc"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
)

var (
	ErrTypeAssert    = errors.New("failed to convert to v1beta2.Descriptor")
	ErrDecode        = errors.New("failed to decode to descriptor target")
	ErrTemplateNil   = errors.New("module template is nil")
	ErrDescriptorNil = errors.New("module template contains nil descriptor")
)

type CachedDescriptorProvider struct {
	DescriptorCache *cache.DescriptorCache
}

func NewCachedDescriptorProvider() *CachedDescriptorProvider {

	return &CachedDescriptorProvider{
		DescriptorCache: cache.NewDescriptorCache(),
	}
}

func (c *CachedDescriptorProvider) GetDescriptor(ociComponentName, componentVersion string) (*types.Descriptor, error) {
	return nil, nil // TODO: implement
}

func (c *CachedDescriptorProvider) GetDescriptorOld(template *v1beta2.ModuleTemplate) (*types.Descriptor, error) {
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

/*
[Review note:]
This method was adding the ComponentDescriptor configured in the given ModuleTemplate to the cache if it is not already present.
We don't need it anymore as now the ComponentDescriptor is fetched directly from the OCI registry.
func (c *CachedDescriptorProvider) Add(template *v1beta2.ModuleTemplate) error {
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
