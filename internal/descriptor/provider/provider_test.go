package provider_test

import (
	"testing"

	//"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	//"ocm.software/ocm/api/ocm/compdesc"

	//"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	//"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	//"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	//"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestGet_OnEmptyIdentity_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	_, err := descriptorProvider.GetDescriptor(ocmidentity.Component{})

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrNameEmpty)

	_, err = descriptorProvider.GetDescriptor(ocmidentity.Component{ComponentName: "name"})

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrVersionEmpty)
}

func TestAdd_OnEmptyIdentity_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	err := descriptorProvider.Add(ocmidentity.Component{})

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrNameEmpty)

	err = descriptorProvider.Add(ocmidentity.Component{ComponentName: "name"})

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrVersionEmpty)
}

/*
TODO: //Fix
func TestGetDescriptor_OnInvalidRawDescriptor_ReturnsErrDescriptorNil(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	template := builder.NewModuleTemplateBuilder().WithRawDescriptor([]byte("invalid descriptor")).WithDescriptor(nil).Build()

	_, err := descriptorProvider.GetDescriptor(template)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrDescriptorNil)
}

func TestGetDescriptor_OnEmptyCache_ReturnsParsedDescriptor(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	template := builder.NewModuleTemplateBuilder().Build()

	_, err := descriptorProvider.GetDescriptor(template)

	require.NoError(t, err)
}

func TestAdd_OnInvalidRawDescriptor_ReturnsErrDecode(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	template := builder.NewModuleTemplateBuilder().WithRawDescriptor([]byte("invalid descriptor")).WithDescriptor(nil).Build()

	err := descriptorProvider.Add(template)

	require.Error(t, err)
	assert.Contains(t, err.Error(), provider.ErrDecode.Error())
}

func TestAdd_OnDescriptorTypeButNull_ReturnsNoError(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	template := builder.NewModuleTemplateBuilder().WithDescriptor(&types.Descriptor{}).Build()

	err := descriptorProvider.Add(template)

	require.NoError(t, err)
}

func TestGetDescriptor_OnEmptyCache_AddsDescriptorFromTemplate(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	descriptorProvider := &provider.CachedDescriptorProvider{
		DescriptorCache: descriptorCache,
	}

	expected := &types.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{
			Metadata: compdesc.Metadata{
				ConfiguredVersion: "v2",
			},
		},
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
*/
