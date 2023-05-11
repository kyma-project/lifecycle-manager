package remote_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/stretchr/testify/assert"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//nolint:funlen
func TestShouldPatchRemoteCRD(t *testing.T) {
	t.Parallel()
	type args struct {
		runtimeCrdGeneration      int64
		kcpCrdGeneration          int64
		runtimeCrdVersion         string
		kymaKcpCrdAnnotationValue string
		kymaSkrCrdAnnotationValue string
		kcpAnnotation             string
		skrAnnotation             string
		err                       error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Different Skr Generation",
			args: args{
				err:                       nil,
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          1,
				runtimeCrdVersion:         v1beta2.GroupVersion.Version,
				kymaKcpCrdAnnotationValue: "1",
				kymaSkrCrdAnnotationValue: "0",
				kcpAnnotation:             v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation:             v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: true,
		},
		{
			name: "Different Kcp Generation",
			args: args{
				err:                       nil,
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          1,
				runtimeCrdVersion:         v1beta2.GroupVersion.Version,
				kymaKcpCrdAnnotationValue: "0",
				kymaSkrCrdAnnotationValue: "1",
				kcpAnnotation:             v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation:             v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: true,
		},
		{
			name: "Same Generations",
			args: args{
				err:                       nil,
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          1,
				runtimeCrdVersion:         v1beta2.GroupVersion.Version,
				kymaKcpCrdAnnotationValue: "1",
				kymaSkrCrdAnnotationValue: "1",
				kcpAnnotation:             v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation:             v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: false,
		},
		{
			name: "Different Version",
			args: args{
				err:                       nil,
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          1,
				runtimeCrdVersion:         "v1alpha1",
				kymaKcpCrdAnnotationValue: "0",
				kymaSkrCrdAnnotationValue: "1",
				kcpAnnotation:             v1beta2.KcpModuleTemplateCRDGenerationAnnotation,
				skrAnnotation:             v1beta2.SkrModuleTemplateCRDGenerationAnnotation,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			runtimeCrd := &v1extensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Generation: testCase.args.runtimeCrdGeneration,
				},
				Spec: v1extensions.CustomResourceDefinitionSpec{
					Versions: []v1extensions.CustomResourceDefinitionVersion{
						{
							Name: testCase.args.runtimeCrdVersion,
						},
					},
				},
			}

			kcpCrd := &v1extensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Generation: testCase.args.kcpCrdGeneration,
				},
			}

			kyma := &v1beta2.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						testCase.args.skrAnnotation: testCase.args.kymaSkrCrdAnnotationValue,
						testCase.args.kcpAnnotation: testCase.args.kymaKcpCrdAnnotationValue,
					},
				},
			}
			assert.Equalf(t, testCase.want, remote.ShouldPatchRemoteCRD(runtimeCrd, kcpCrd, kyma, testCase.args.kcpAnnotation,
				testCase.args.skrAnnotation, testCase.args.err),
				"ShouldPatchRemoteCRD(%v, %v, %v, %v, %v, %v)", runtimeCrd, kcpCrd, kyma, testCase.args.kcpAnnotation,
				testCase.args.skrAnnotation, testCase.args.err)
		})
	}
}
