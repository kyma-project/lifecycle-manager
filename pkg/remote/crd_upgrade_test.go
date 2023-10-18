package remote_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
)

//nolint:funlen
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
			runtimeCrd := &apiextensions.CustomResourceDefinition{
				ObjectMeta: apimachinerymeta.ObjectMeta{
					Generation: testCase.args.runtimeCrdGeneration,
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Names: apiextensions.CustomResourceDefinitionNames{
						Kind: "ModuleTemplate",
					},
				},
			}

			kcpCrd := &apiextensions.CustomResourceDefinition{
				ObjectMeta: apimachinerymeta.ObjectMeta{
					Generation: testCase.args.kcpCrdGeneration,
				},
				Spec: apiextensions.CustomResourceDefinitionSpec{
					Names: apiextensions.CustomResourceDefinitionNames{
						Kind: "ModuleTemplate",
					},
				},
			}

			kyma := &v1beta2.Kyma{
				ObjectMeta: apimachinerymeta.ObjectMeta{
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
