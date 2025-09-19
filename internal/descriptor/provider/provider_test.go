package provider_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ocm.software/ocm/api/ocm/compdesc"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestGetDescriptor_OnEmptySpec_ReturnsErrDecode(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider() // assuming it handles nil cache internally
	template := &v1beta2.ModuleTemplate{}

	_, err := descriptorProvider.GetDescriptor(template)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrDecode)
}

func TestAdd_OnNilTemplate_ReturnsErrTemplateNil(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()

	err := descriptorProvider.Add(nil)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrTemplateNil)
}

func TestGetDescriptor_OnNilTemplate_ReturnsErrTemplateNil(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()

	_, err := descriptorProvider.GetDescriptor(nil)

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrTemplateNil)
}

func TestGetDescriptor_OnInvalidRawDescriptor_ReturnsErrDescriptorNil(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	template := builder.NewModuleTemplateBuilder().
		WithRawDescriptor([]byte("invalid descriptor")).
		WithDescriptor(nil).
		Build()

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
	template := builder.NewModuleTemplateBuilder().
		WithVersion("1.0.0").
		WithRawDescriptor([]byte("invalid descriptor")).
		WithDescriptor(nil).
		Build()

	err := descriptorProvider.Add(template)

	require.Error(t, err)
	assert.Contains(t, err.Error(), provider.ErrDecode.Error())
}

func TestAdd_OnDescriptorTypeButNull_ReturnsNoError(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider()
	template := builder.NewModuleTemplateBuilder().WithVersion("1.0.0").WithDescriptor(&types.Descriptor{}).Build()

	err := descriptorProvider.Add(template)

	require.NoError(t, err)
}

func TestGetDescriptor_OnEmptyCache_AddsDescriptorFromTemplate(t *testing.T) {
	descriptorCache := cache.NewDescriptorCache()
	descriptorProvider := &provider.CachedDescriptorProvider{
		DescriptorCache: descriptorCache,
	}

	expected := &types.Descriptor{
		ComponentDescriptor: compdesc.New("test-component", "1.0.0"),
	}
	template := builder.NewModuleTemplateBuilder().WithVersion("1.0.0").WithDescriptor(expected).Build()

	key := descriptorProvider.GenerateDescriptorKey(template.Name, template.GetVersion())
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

func TestGenerateDescriptorCacheKey(t *testing.T) {
	testCases := []struct {
		name          string
		moduleName    string
		moduleVersion string
		want          string
	}{
		{
			name:          "with valid module name and version",
			moduleName:    "name",
			moduleVersion: "1.0.0",
			want:          "name:1.0.0",
		},
	}

	providerInstance := provider.NewCachedDescriptorProvider()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := providerInstance.GenerateDescriptorKey(tc.moduleName, tc.moduleVersion)
			assert.Equal(t, tc.want, got)
		})
	}
}
