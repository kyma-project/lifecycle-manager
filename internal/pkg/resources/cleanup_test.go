package resources_test

import (
	"reflect"
	"testing"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apirbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/resources"
)

func Test_IsOperatorRelatedResources(t *testing.T) {
	type args struct {
		kind any
	}
	tests := []struct {
		name string
		args []args
		want bool
	}{
		{
			"operator related resources should be determined",
			[]args{
				{kind: apicorev1.ServiceAccount{}},
				{kind: apicorev1.Service{}},
				{kind: apirbacv1.Role{}},
				{kind: apirbacv1.ClusterRole{}},
				{kind: apirbacv1.RoleBinding{}},
				{kind: apirbacv1.ClusterRoleBinding{}},
				{kind: apiappsv1.Deployment{}},
				{kind: apiextensionsv1.CustomResourceDefinition{}},
			},
			true,
		},
		{
			"non operator related resources should be ignored",
			[]args{
				{kind: apicorev1.Pod{}},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, arg := range tt.args {
				if got := resources.IsOperatorRelatedResources(getKindName(arg.kind)); got != tt.want {
					t.Errorf("IsOperatorRelatedResources() = %v, want %v", got, tt.want)
				}
			}

		})
	}
}

func getKindName(cr any) string {
	return reflect.TypeOf(cr).Name()
}
