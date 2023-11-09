package v1beta2_test

import (
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

//nolint:funlen
func TestModuleTemplate_GetComponentDescriptorCacheKey(t *testing.T) {
	t.Parallel()

	type fields struct {
		TypeMeta   apimetav1.TypeMeta
		ObjectMeta apimetav1.ObjectMeta
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
				ObjectMeta: apimetav1.ObjectMeta{
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
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "test-module-without-version",
					Generation: 2,
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "regular",
				},
			},
			want: "test-module-without-version:regular:2",
		},
		{
			name: "ModuleTemplate without version annotation but with other annotations",
			fields: fields{
				ObjectMeta: apimetav1.ObjectMeta{
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
		{
			name: "ModuleTemplate with invalid version annotation ",
			fields: fields{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "test-module-with-invalid-version",
					Generation: 2,
					Annotations: map[string]string{
						v1beta2.IsClusterScopedAnnotation: "true",
						v1beta2.ModuleVersionAnnotation:   "a",
					},
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "regular",
				},
			},
			want: "test-module-with-invalid-version:regular:2",
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
