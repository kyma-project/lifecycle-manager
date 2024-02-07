package provider_test

import (
	"testing"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestGetDescriptor_OnEmptySpec_ReturnsErrDecode(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil) // assuming it handles nil cache internally
	template := &v1beta2.ModuleTemplate{}

	_, err := descriptorProvider.GetDescriptor(template)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrDecode)
}

func TestAdd_OnNilTemplate_ReturnsErrTemplateNil(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil)

	err := descriptorProvider.Add(nil)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrTemplateNil)
}

func TestGetDescriptor_OnNilTemplate_ReturnsErrTemplateNil(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil)

	_, err := descriptorProvider.GetDescriptor(nil)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrTemplateNil)
}

func TestGetDescriptor_DecodeError(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil)
	template := builder.NewModuleTemplateBuilder().WithRawDescriptor([]byte("invalid descriptor")).WithDescriptor(nil).Build()

	_, err := descriptorProvider.GetDescriptor(template)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrDescriptorNil)
}

func TestGetDescriptor_OnEmptyCache_ReturnsParsedDescriptor(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	descriptorProvider := provider.NewCachedDescriptorProvider(descriptorCache)
	template := builder.NewModuleTemplateBuilder().Build()

	_, err := descriptorProvider.GetDescriptor(template)

	require.NoError(t, err)
}

func TestAdd_DecodeError(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil)
	template := builder.NewModuleTemplateBuilder().WithRawDescriptor([]byte("invalid descriptor")).WithDescriptor(nil).Build()

	err := descriptorProvider.Add(template)

	require.Error(t, err)
	assert.Contains(t, err.Error(), provider.ErrDecode.Error())
}

func TestGetDescriptor_OnEmptyCache_AddsDescriptorFromTemplate(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	descriptorProvider := provider.NewCachedDescriptorProvider(descriptorCache)

	expected := &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{Metadata: compdesc.Metadata{
			ConfiguredVersion: "v2",
		}},
	}
	template := builder.NewModuleTemplateBuilder().WithDescriptor(expected).Build()

	key := cache.GenerateDescriptorKey(template)
	entry := descriptorCache.Get(key)
	assert.Nil(t, entry)

	err := descriptorProvider.Add(template)
	require.NoError(t, err)

	result, err := descriptorProvider.GetDescriptor(template)
	require.NoError(t, err)
	assert.Equal(t, expected.Name, result.Name)

	entry = descriptorCache.Get(key)
	assert.NotNil(t, entry)
	assert.Equal(t, expected.Name, entry.Name)
}
