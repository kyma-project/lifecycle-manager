package img_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
	"github.com/kyma-project/lifecycle-manager/internal/service/componentdescriptor"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name                 string
		descriptorSourceFile string
		descriptorVersion    string
		want                 img.Layer
	}{
		{
			"should parse raw-manifest layer from mediaType: application/x-tar",
			"v1beta2_template_operator_new_ocm.yaml",
			"1.0.0-new-ocm-format",
			img.Layer{
				LayerName: "raw-manifest",
				LayerRepresentation: &img.OCI{
					Repo: "europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/template-operator/component-descriptors",
					Name: testutils.DefaultFQDN,
					Ref:  "sha256:d2cc278224a71384b04963a83e784da311a268a2b3fa8732bc31e70ca0c5bc52",
					Type: "oci-dir",
				},
			},
		}, {
			"should parse raw-manifest layer from mediaType: application/octet-stream",
			"v1beta2_template_operator_current_ocm.yaml",
			"1.1.1-e2e-test",
			img.Layer{
				LayerName: "raw-manifest",
				LayerRepresentation: &img.OCI{
					Repo: "europe-west3-docker.pkg.dev/sap-kyma-jellyfish-dev/template-operator/component-descriptors",
					Name: testutils.DefaultFQDN,
					Ref:  "sha256:1ea2baf45791beafabfee533031b715af8f7a4ffdfbbf30d318f52f7652c36ca",
					Type: "oci-ref",
				},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var moduleTemplateFromFile v1beta2.ModuleTemplate
			builder.ReadComponentDescriptorFromFile(testCase.descriptorSourceFile,
				&moduleTemplateFromFile)
			ocmId, err := ocmidentity.NewComponentId(
				"kyma-project.io/module/template-operator", testCase.descriptorVersion)
			require.NoError(t, err)
			descriptor, err := provider.NewCachedDescriptorProvider(
				componentdescriptor.NewFakeService(moduleTemplateFromFile.Spec.Descriptor.Raw)).
				GetDescriptor(*ocmId)
			require.NoError(t, err)
			layers, err := img.Parse(descriptor.ComponentDescriptor)
			require.NoError(t, err)
			for _, layer := range layers {
				if layer.LayerName == testCase.want.LayerName {
					require.Equal(t, testCase.want.LayerRepresentation, layer.LayerRepresentation)
				}
			}
		})
	}
}
