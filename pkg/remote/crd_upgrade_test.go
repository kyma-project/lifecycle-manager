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
		kymaKcpCrdAnnotationValue string
		kymaSkrCrdAnnotationValue string
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
				kymaKcpCrdAnnotationValue: "1",
				kymaSkrCrdAnnotationValue: "0",
			},
			want: true,
		},
		{
			name: "Different Kcp Generation",
			args: args{
				err:                       nil,
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          1,
				kymaKcpCrdAnnotationValue: "0",
				kymaSkrCrdAnnotationValue: "1",
			},
			want: true,
		},
		{
			name: "Same Generations",
			args: args{
				err:                       nil,
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          1,
				kymaKcpCrdAnnotationValue: "1",
				kymaSkrCrdAnnotationValue: "1",
			},
			want: false,
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
					Names: v1extensions.CustomResourceDefinitionNames{
						Kind: "ModuleTemplate",
					},
				},
			}

			kcpCrd := &v1extensions.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Generation: testCase.args.kcpCrdGeneration,
				},
				Spec: v1extensions.CustomResourceDefinitionSpec{
					Names: v1extensions.CustomResourceDefinitionNames{
						Kind: "ModuleTemplate",
					},
				},
			}

			kyma := &v1beta2.Kyma{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1beta2.SkrModuleTemplateCRDGenerationAnnotation: testCase.args.kymaSkrCrdAnnotationValue,
						v1beta2.KcpModuleTemplateCRDGenerationAnnotation: testCase.args.kymaKcpCrdAnnotationValue,
					},
				},
			}
			assert.Equalf(t, testCase.want, remote.ShouldPatchRemoteCRD(runtimeCrd, kcpCrd, kyma, testCase.args.err),
				"ShouldPatchRemoteCRD(%v, %v, %v, %v)", runtimeCrd, kcpCrd, kyma, testCase.args.err)
		})
	}
}
