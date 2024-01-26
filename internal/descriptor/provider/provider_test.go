package provider_test

import (
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestCachedDescriptorProvider(t *testing.T) {
	sut := provider.NewCachedDescriptorProvider()

	testDescriptor := &v1beta2.Descriptor{
		ComponentDescriptor: &compdesc.ComponentDescriptor{},
	}
	testTemplate := &v1beta2.ModuleTemplate{
		Spec: v1beta2.ModuleTemplateSpec{
			Descriptor: machineryruntime.RawExtension{
				Object: testDescriptor,
			},
		},
	}

	// ensure the internal cache is initially empty
	assert.Equal(t, false, sut.IsCached(testTemplate))

	// manually add a descriptor from template
	err := sut.Add(testTemplate)
	assert.NoError(t, err)

	// now the internal cache should not be empty
	assert.Equal(t, true, sut.IsCached(testTemplate))

	// getting descriptor from provider as expected
	descriptor, err := sut.GetDescriptor(testTemplate)
	assert.NoError(t, err)
	assert.Equal(t, descriptor.Name, testDescriptor.Name)
}
