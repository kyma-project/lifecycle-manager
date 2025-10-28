package provider_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	descriptorcache "github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/service/componentdescriptor"
)

func TestGetDescriptor_OnEmptyIdentity_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil, nil)
	_, err := descriptorProvider.GetDescriptor(ocmidentity.ComponentId{})

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrNameOrVersionEmpty)
}

func TestAdd_OnEmptyIdentity_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil, nil)
	err := descriptorProvider.Add(ocmidentity.ComponentId{})

	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrNameOrVersionEmpty)
}

func TestGetDescriptor_OnInvalidRawDescriptor_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(
		(&componentdescriptor.FakeService{}).Register([]byte("invalid descriptor")),
		descriptorcache.NewDescriptorCache(),
	)
	ocmId, err := ocmidentity.NewComponentId("test", "v1")
	require.NoError(t, err)
	_, err = descriptorProvider.GetDescriptor(*ocmId)

	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrDecode)
}

func TestGetDescriptor_OnEmptyCache_ReturnsDescriptorFromService(t *testing.T) {
	// given
	var moduleTemplateFromFile v1beta2.ModuleTemplate
	builder.ReadComponentDescriptorFromFile("v1beta2_template_operator_new_ocm.yaml", &moduleTemplateFromFile)

	descriptorProvider := provider.NewCachedDescriptorProvider(
		(&componentdescriptor.FakeService{}).Register(moduleTemplateFromFile.Spec.Descriptor.Raw),
		descriptorcache.NewDescriptorCache(),
	)

	ocmId, err := ocmidentity.NewComponentId("kyma-project.io/module/template-operator", "1.0.0-new-ocm-format")
	require.NoError(t, err)

	// when
	desc, err := descriptorProvider.GetDescriptor(*ocmId)

	// then
	require.NoError(t, err)
	assert.Equal(t, ocmId.Name(), desc.Name)
	assert.Equal(t, ocmId.Version(), desc.Version)
}

func TestGetDescriptor_DoesNotUpdateCache(t *testing.T) {
	// given
	var moduleTemplateFromFile v1beta2.ModuleTemplate
	builder.ReadComponentDescriptorFromFile("v1beta2_template_operator_new_ocm.yaml", &moduleTemplateFromFile)

	mockService := &componentdescriptor.FakeService{}
	mockService.Register(moduleTemplateFromFile.Spec.Descriptor.Raw)

	descriptorProvider := provider.NewCachedDescriptorProvider(mockService, descriptorcache.NewDescriptorCache())

	ocmId, err := ocmidentity.NewComponentId("kyma-project.io/module/template-operator", "1.0.0-new-ocm-format")
	require.NoError(t, err)

	// when
	desc, err := descriptorProvider.GetDescriptor(*ocmId)

	// then
	require.NoError(t, err)
	assert.Equal(t, ocmId.Name(), desc.Name)
	assert.Equal(t, ocmId.Version(), desc.Version)

	// and when
	mockService.Clear().Register([]byte("invalid descriptor")) // make the service return junk data
	_, err = descriptorProvider.GetDescriptor(*ocmId)          // should come from the service,
	//                                                            because the cache was not updated - and fail

	// then
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrDecode)
}

func TestAddDescriptor_DoesNotUpdateCacheIfKeyExists(t *testing.T) {
	// given
	mockCache := &mockCache{
		result: &types.Descriptor{},
	}

	descriptorProvider := provider.NewCachedDescriptorProvider(nil, mockCache)

	ocmId, err := ocmidentity.NewComponentId("kyma-project.io/module/template-operator", "1.0.0-new-ocm-format")
	require.NoError(t, err)

	// when
	err = descriptorProvider.Add(*ocmId)

	// then
	require.NoError(t, err)
}

func TestAddDescriptor_ReturnsErrorFromService(t *testing.T) {
	// given
	expectedErr := errors.New("mock error")
	mockCache := &mockCache{}
	mockService := &componentdescriptor.FakeService{
		GetError: expectedErr,
	}

	descriptorProvider := provider.NewCachedDescriptorProvider(mockService, mockCache)

	ocmId, err := ocmidentity.NewComponentId("kyma-project.io/module/template-operator", "1.0.0-new-ocm-format")
	require.NoError(t, err)

	// when
	err = descriptorProvider.Add(*ocmId)

	// then
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	assert.Contains(t, err.Error(), "error finding ComponentDescriptor: mock error")
}

func TestGetDescriptor_ReturnsDescriptorFromCache(t *testing.T) {
	// given
	var moduleTemplateFromFile v1beta2.ModuleTemplate
	builder.ReadComponentDescriptorFromFile("v1beta2_template_operator_new_ocm.yaml", &moduleTemplateFromFile)
	mockService := &componentdescriptor.FakeService{}
	mockService.Register(moduleTemplateFromFile.Spec.Descriptor.Raw)
	descriptorProvider := provider.NewCachedDescriptorProvider(mockService, descriptorcache.NewDescriptorCache())
	ocmId, err := ocmidentity.NewComponentId("kyma-project.io/module/template-operator", "1.0.0-new-ocm-format")
	require.NoError(t, err)

	err = descriptorProvider.Add(*ocmId) // add to cache
	require.NoError(t, err)

	// when
	mockService.Clear().Register([]byte("invalid descriptor"))     // make the service return junk data
	descFromCache, err := descriptorProvider.GetDescriptor(*ocmId) // should come from the cache

	// then
	require.NoError(t, err)
	assert.Equal(t, ocmId.Name(), descFromCache.Name)
	assert.Equal(t, ocmId.Version(), descFromCache.Version)
}

func TestGetDescriptorWithIdentity_WithNilProvider_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil, nil)
	_, err := descriptorProvider.GetDescriptorWithIdentity(nil)
	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrNilProvider)
}

func TestGetDescriptorWithIdentity_WithNilIdentity_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil, nil)
	_, err := descriptorProvider.GetDescriptorWithIdentity(&mockIdentityProvider{})
	require.Error(t, err)
	require.ErrorIs(t, err, provider.ErrNilIdentity)
}

func TestGetDescriptorWithIdentity_WithProviderErr_ReturnsErr(t *testing.T) {
	descriptorProvider := provider.NewCachedDescriptorProvider(nil, nil)
	expectedErr := errors.New("some error")
	_, err := descriptorProvider.GetDescriptorWithIdentity(
		&mockIdentityProvider{err: expectedErr})
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
}

func TestGetDescriptorWithIdentity_OnValidIdentity_ReturnsDescriptor(t *testing.T) {
	// given
	var moduleTemplateFromFile v1beta2.ModuleTemplate
	builder.ReadComponentDescriptorFromFile("v1beta2_template_operator_new_ocm.yaml", &moduleTemplateFromFile)

	descriptorProvider := provider.NewCachedDescriptorProvider(
		(&componentdescriptor.FakeService{}).Register(moduleTemplateFromFile.Spec.Descriptor.Raw),
		descriptorcache.NewDescriptorCache(),
	)

	ocmId, err := ocmidentity.NewComponentId("kyma-project.io/module/template-operator", "1.0.0-new-ocm-format")
	require.NoError(t, err)
	mockProvider := &mockIdentityProvider{ocmId: ocmId}

	// when
	desc, err := descriptorProvider.GetDescriptorWithIdentity(mockProvider)

	// then
	require.NoError(t, err)
	assert.Equal(t, ocmId.Name(), desc.Name)
	assert.Equal(t, ocmId.Version(), desc.Version)
}

type mockIdentityProvider struct {
	err   error
	ocmId *ocmidentity.ComponentId
}

func (b *mockIdentityProvider) GetOCMIdentity() (*ocmidentity.ComponentId, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.ocmId, nil
}

type mockCache struct {
	result *types.Descriptor
}

func (m *mockCache) Get(key descriptorcache.DescriptorKey) *types.Descriptor {
	if m.result != nil {
		return m.result
	}
	return nil
}

func (m *mockCache) Set(key descriptorcache.DescriptorKey, value *types.Descriptor) {
	m.result = value
}
