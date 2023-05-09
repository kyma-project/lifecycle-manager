package remote_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/stretchr/testify/assert"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestUpdateKymaAnnotations(t *testing.T) {
	type args struct {
		kyma              *v1beta2.Kyma
		kcpCRD            *v1extensions.CustomResourceDefinition
		skrCRD            *v1extensions.CustomResourceDefinition
		kcpAnnotationName string
		skrAnnotationName string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Update ModuleTemplate Annotations",
			args: args{
				kyma: testutils.NewTestKyma("test-kyma"),
				skrCRD: &v1extensions.CustomResourceDefinition{
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Names: v1extensions.CustomResourceDefinitionNames{
							Kind: string(v1beta2.ModuleTemplateKind),
						},
					},
				},
				kcpCRD: &v1extensions.CustomResourceDefinition{
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Names: v1extensions.CustomResourceDefinitionNames{
							Kind: string(v1beta2.ModuleTemplateKind),
						},
					},
				},
				kcpAnnotationName: v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotationName: v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
		},
		{
			name: "Update Kyma Annotations",
			args: args{
				kyma: testutils.NewTestKyma("test-kyma"),
				skrCRD: &v1extensions.CustomResourceDefinition{
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Names: v1extensions.CustomResourceDefinitionNames{
							Kind: string(v1beta2.KymaKind),
						},
					},
				},
				kcpCRD: &v1extensions.CustomResourceDefinition{
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Names: v1extensions.CustomResourceDefinitionNames{
							Kind: string(v1beta2.KymaKind),
						},
					},
				},
				kcpAnnotationName: v1beta2.KcpKymaCRDGenerationAnnotation,
				skrAnnotationName: v1beta2.SkrKymaCRDGenerationAnnotation,
			},
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.args.skrCRD.SetGeneration(2)
			tt.args.kcpCRD.SetGeneration(1)
			remote.UpdateKymaAnnotations(tt.args.kyma, tt.args.kcpCRD, tt.args.skrCRD)
			assert.Equal(t, "1", tt.args.kyma.Annotations[tt.args.kcpAnnotationName])
			assert.Equal(t, "2", tt.args.kyma.Annotations[tt.args.skrAnnotationName])
		})
	}
}
