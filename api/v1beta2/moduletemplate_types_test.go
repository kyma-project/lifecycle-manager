package v1beta2_test

import (
	"fmt"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestModuleTemplate_GetComponentDescriptorCacheKey(t *testing.T) {
	t.Parallel()

	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       v1beta2.ModuleTemplateSpec
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "ModuleTemplate with version annotation",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						v1beta2.ModuleVersionAnnotation: "1.1.0",
					},
					Name:       "test-module-with-version",
					Generation: 2,
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "regular",
				},
			},
			want: "test-module-with-version:regular:1.1.0",
		},
		{
			name: "ModuleTemplate without version annotation",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:       "test-module-without-version",
					Generation: 2,
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "regular",
				},
			},
			want: fmt.Sprintf("%s:%s:%d", "test-module-without-version", "regular", 2),
		},
		{
			name: "ModuleTemplate without version annotation but with other annotations",
			fields: fields{
				ObjectMeta: v1.ObjectMeta{
					Name:       "test-module-without-version",
					Generation: 2,
					Annotations: map[string]string{
						v1beta2.IsClusterScopedAnnotation: "true",
					},
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "regular",
				},
			},
			want: "test-module-without-version:regular:2",
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			m := &v1beta2.ModuleTemplate{
				TypeMeta:   testCase.fields.TypeMeta,
				ObjectMeta: testCase.fields.ObjectMeta,
				Spec:       testCase.fields.Spec,
			}
			if got := m.GetComponentDescriptorCacheKey(); got != testCase.want {
				t.Errorf("GetComponentDescriptorCacheKey() = %v, want %v", got, testCase.want)
			}
		})
	}
}
