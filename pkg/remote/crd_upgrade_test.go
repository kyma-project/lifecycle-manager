package remote_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
)

func TestShouldPatchRemoteCRD(t *testing.T) {
	t.Parallel()
	type args struct {
		runtimeCrdGeneration      int64
		kcpCrdGeneration          int64
		kymaKcpCrdAnnotationValue string
		kymaSkrCrdAnnotationValue string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Different Skr Generation",
			args{
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          2,
				kymaKcpCrdAnnotationValue: "1",
				kymaSkrCrdAnnotationValue: "1",
			},
			true,
		},
		{
			"Different Kcp Generation",
			args{
				runtimeCrdGeneration:      2,
				kcpCrdGeneration:          1,
				kymaKcpCrdAnnotationValue: "1",
				kymaSkrCrdAnnotationValue: "1",
			},
			true,
		},
		{
			"Same Generations",
			args{
				runtimeCrdGeneration:      1,
				kcpCrdGeneration:          1,
				kymaKcpCrdAnnotationValue: "1",
				kymaSkrCrdAnnotationValue: "1",
			},
			false,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			runtimeCrd := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: apimetav1.ObjectMeta{
					Generation: testCase.args.runtimeCrdGeneration,
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind: "ModuleTemplate",
					},
				},
			}

			kcpCrd := &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: apimetav1.ObjectMeta{
					Generation: testCase.args.kcpCrdGeneration,
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{
						Kind: "ModuleTemplate",
					},
				},
			}

			kyma := &v1beta2.Kyma{
				ObjectMeta: apimetav1.ObjectMeta{
					Annotations: map[string]string{
						"moduletemplate-skr-crd-generation": testCase.args.kymaSkrCrdAnnotationValue,
						"moduletemplate-kcp-crd-generation": testCase.args.kymaKcpCrdAnnotationValue,
					},
				},
			}
			assert.Equalf(t, testCase.want, remote.ShouldPatchRemoteCRD(runtimeCrd, kcpCrd, kyma),
				"ShouldPatchRemoteCRD(%v, %v, %v, %v)", runtimeCrd, kcpCrd, kyma)
		})
	}
}
