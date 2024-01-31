package provider

import (
	"errors"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
)

var (
	errTypeAssert  = errors.New("failed to convert to v1beta2.Descriptor")
	errDecode      = errors.New("failed to decode to descriptor target")
	errTemplateNil = errors.New("module template is nil")
)

type CachedDescriptorProvider struct {
	descriptorCache *cache.DescriptorCache
}

func NewCachedDescriptorProvider(descriptorCache *cache.DescriptorCache) *CachedDescriptorProvider {
	if descriptorCache != nil {
		return &CachedDescriptorProvider{
			descriptorCache: descriptorCache,
		}
	}
	return &CachedDescriptorProvider{
		descriptorCache: cache.NewDescriptorCache(),
	}
}

func (c *CachedDescriptorProvider) GetDescriptor(template *v1beta2.ModuleTemplate) (*v1beta2.Descriptor, error) {
	if template.Spec.Descriptor.Object != nil {
		desc, ok := template.Spec.Descriptor.Object.(*v1beta2.Descriptor)
		if !ok {
			return nil, errTypeAssert
		}
		return desc, nil
	}
	key := cache.GenerateDescriptorKey(template)
	descriptor := c.descriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	ocmDesc, err := compdesc.Decode(
		template.Spec.Descriptor.Raw, []compdesc.DecodeOption{compdesc.DisableValidation(true)}...,
	)
	if err != nil {
		return nil, errors.Join(errDecode, err)
	}

	template.Spec.Descriptor.Object = &v1beta2.Descriptor{ComponentDescriptor: ocmDesc}
	descriptor, ok := template.Spec.Descriptor.Object.(*v1beta2.Descriptor)
	if !ok {
		return nil, errTypeAssert
	}

	return descriptor, nil
}

func (c *CachedDescriptorProvider) Add(template *v1beta2.ModuleTemplate) error {
	if template == nil {
		return errTemplateNil
	}
	key := cache.GenerateDescriptorKey(template)
	descriptor := c.descriptorCache.Get(key)
	if descriptor != nil {
		return nil
	}

	if template.Spec.Descriptor.Object != nil {
		desc, ok := template.Spec.Descriptor.Object.(*v1beta2.Descriptor)
		if ok && desc != nil {
			c.descriptorCache.Set(key, desc)
			return nil
		}
	}

	ocmDesc, err := compdesc.Decode(
		template.Spec.Descriptor.Raw, []compdesc.DecodeOption{compdesc.DisableValidation(true)}...,
	)
	if err != nil {
		return errors.Join(errDecode, err)
	}

	template.Spec.Descriptor.Object = &v1beta2.Descriptor{ComponentDescriptor: ocmDesc}
	descriptor, ok := template.Spec.Descriptor.Object.(*v1beta2.Descriptor)
	if !ok {
		return errTypeAssert
	}

	c.descriptorCache.Set(key, descriptor)
	return nil
}
