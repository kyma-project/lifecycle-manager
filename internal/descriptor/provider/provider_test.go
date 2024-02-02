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

func TestGetDescriptor_OnEmptyCache_ReturnsParsedDescriptor(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	sut := provider.NewCachedDescriptorProvider(descriptorCache)
	template := builder.NewModuleTemplateBuilder().Build()
	_, err := sut.GetDescriptor(template)

	require.NoError(t, err)
}

func TestGetDescriptor_OnEmptyCache_AddsDescriptorFromTemplate(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	sut := provider.NewCachedDescriptorProvider(descriptorCache)

	expected := &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{Metadata: compdesc.Metadata{
			ConfiguredVersion: "v2",
		}},
	}
	template := builder.NewModuleTemplateBuilder().WithDescriptor(expected).Build()

	key := cache.GenerateDescriptorKey(template)
	entry := descriptorCache.Get(key)
	assert.Nil(t, entry)

	err := sut.Add(template)
	require.NoError(t, err)

	result, err := sut.GetDescriptor(template)
	require.NoError(t, err)
	assert.Equal(t, expected.Name, result.Name)

	entry = descriptorCache.Get(key)
	assert.NotNil(t, entry)
	assert.Equal(t, expected.Name, entry.Name)
}
