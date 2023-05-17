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
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Different Skr Generation",
			args{
				1,
				2,
				"1",
				"1",
			},
			true,
		},
		{
			"Different Kcp Generation",
			args{
				2,
				1,
				"1",
				"1",
			},
			true,
		},
		{
			"Same Generations",
			args{
				1,
				1,
				"1",
				"1",
			},
			false,
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
						"moduletemplate-skr-crd-generation": testCase.args.kymaSkrCrdAnnotationValue,
						"moduletemplate-kcp-crd-generation": testCase.args.kymaKcpCrdAnnotationValue,
					},
				},
			}
			assert.Equalf(t, testCase.want, remote.ShouldPatchRemoteCRD(runtimeCrd, kcpCrd, kyma),
				"ShouldPatchRemoteCRD(%v, %v, %v)", runtimeCrd, kcpCrd, kyma)
		})
	}
}
