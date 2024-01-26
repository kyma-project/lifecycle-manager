package provider_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestCachedDescriptorProvider(t *testing.T) {
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
	assert.NoError(t, err)

	// now the internal cache should not be empty
	assert.True(t, sut.IsCached(template))

	// getting the descriptor from provider as expected
	actual, err := sut.GetDescriptor(template)
	assert.NoError(t, err)
	assert.Equal(t, expected.Name, actual.Name)
}
