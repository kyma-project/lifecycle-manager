package provider

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
)

var (
	errTypeAssert  = errors.New("failed to convert to v1beta2.Descriptor")
	errDecode      = errors.New("failed to decode to descriptor target")
	errTemplateNil = errors.New("module template is nil")
)

type CachedDescriptorProvider struct {
	descriptorCache *cache.DescriptorCache
}

// TODO inject decoder to abstract away OCM dep

func NewCachedDescriptorProvider() *CachedDescriptorProvider {
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
	key := cache.GenerateDescriptorCacheKey(template)
	descriptor := c.descriptorCache.Get(key)
	if descriptor != nil {
		return descriptor, nil
	}

	// TODO use injected decoder
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
	key := cache.GenerateDescriptorCacheKey(template)
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

	// TODO use injected decoder
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

func (c *CachedDescriptorProvider) IsCached(template *v1beta2.ModuleTemplate) bool {
	if template == nil {
		return false
	}
	key := cache.GenerateDescriptorCacheKey(template)
	descriptor := c.descriptorCache.Get(key)
	return descriptor != nil
}
