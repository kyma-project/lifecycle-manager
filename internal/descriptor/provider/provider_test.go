package provider_test

import (
	"testing"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
)

func TestCachedDescriptorProvider(t *testing.T) {
	t.Parallel()

	sut := provider.NewCachedDescriptorProvider()
	expected := &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{},
	}
	template := &v1beta2.ModuleTemplate{
		Spec: v1beta2.ModuleTemplateSpec{
			Descriptor: machineryruntime.RawExtension{
				Object: expected,
			},
		},
	}

	// ensure the internal cache is initially empty
	assert.False(t, sut.IsCached(template))

	// manually add a descriptor from template
	err := sut.Add(template)
	require.NoError(t, err)

	// now the internal cache should not be empty
	assert.True(t, sut.IsCached(template))

	// getting the descriptor from provider as expected
	actual, err := sut.GetDescriptor(template)
	require.NoError(t, err)
	assert.Equal(t, expected.Name, actual.Name)
}
