package remote

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/stretchr/testify/assert"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldPatchRemoteCRD(t *testing.T) {
	type args struct {
		runtimeCrd    *v1extensions.CustomResourceDefinition
		kcpCrd        *v1extensions.CustomResourceDefinition
		kyma          *v1beta2.Kyma
		kcpAnnotation string
		skrAnnotation string
		err           error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Different Skr Generation",
			args: args{
				err: nil,
				runtimeCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Versions: []v1extensions.CustomResourceDefinitionVersion{
							{
								Name: v1beta2.GroupVersion.Version,
							},
						},
					},
				},
				kcpCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
				kyma: &v1beta2.Kyma{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1beta2.SkrModuleTemplateCRDGenerationAnnotation: "0",
							v1beta2.KcpModuleTemplateCRDGenerationAnnotation: "1",
						},
					},
				},
				kcpAnnotation: v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation: v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: true,
		},
		{
			name: "Different Kcp Generation",
			args: args{
				err: nil,
				runtimeCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Versions: []v1extensions.CustomResourceDefinitionVersion{
							{
								Name: v1beta2.GroupVersion.Version,
							},
						},
					},
				},
				kcpCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
				kyma: &v1beta2.Kyma{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1beta2.SkrModuleTemplateCRDGenerationAnnotation: "1",
							v1beta2.KcpModuleTemplateCRDGenerationAnnotation: "0",
						},
					},
				},
				kcpAnnotation: v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation: v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: true,
		},
		{
			name: "Same Generations",
			args: args{
				err: nil,
				runtimeCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Versions: []v1extensions.CustomResourceDefinitionVersion{
							{
								Name: v1beta2.GroupVersion.Version,
							},
						},
					},
				},
				kcpCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
				kyma: &v1beta2.Kyma{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1beta2.SkrModuleTemplateCRDGenerationAnnotation: "1",
							v1beta2.KcpModuleTemplateCRDGenerationAnnotation: "1",
						},
					},
				},
				kcpAnnotation: v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation: v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: false,
		},
		{
			name: "Different Version",
			args: args{
				err: nil,
				runtimeCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Spec: v1extensions.CustomResourceDefinitionSpec{
						Versions: []v1extensions.CustomResourceDefinitionVersion{
							{
								Name: "v1alpha1",
							},
						},
					},
				},
				kcpCrd: &v1extensions.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
				kyma: &v1beta2.Kyma{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1beta2.SkrModuleTemplateCRDGenerationAnnotation: "1",
							v1beta2.KcpModuleTemplateCRDGenerationAnnotation: "1",
						},
					},
				},
				kcpAnnotation: v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation: v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ShouldPatchRemoteCRD(tt.args.runtimeCrd, tt.args.kcpCrd, tt.args.kyma, tt.args.kcpAnnotation, tt.args.skrAnnotation, tt.args.err), "ShouldPatchRemoteCRD(%v, %v, %v, %v, %v, %v)", tt.args.runtimeCrd, tt.args.kcpCrd, tt.args.kyma, tt.args.kcpAnnotation, tt.args.skrAnnotation, tt.args.err)
		})
	}
}
