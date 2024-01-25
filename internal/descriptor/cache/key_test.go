package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
)

func TestGenerateDescriptorCacheKey(t *testing.T) {
	testCases := []struct {
		name     string
		template *v1beta2.ModuleTemplate
		want     cache.DescriptorCacheKey
	}{
		{
			name: "Annotations is not nil and valid semver",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: "name",
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: "1.0.0",
					}},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "channel",
				},
			},
			want: "name:channel:1.0.0",
		},
		{
			name: "Annotations is not nil but invalid semver",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "name",
					Generation: 1,
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: "not-semver",
					}},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "channel",
				},
			},
			want: "name:channel:1",
		},
		{
			name: "Annotations is not nil but module version is empty",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "name",
					Generation: 2,
					Annotations: map[string]string{
						shared.ModuleVersionAnnotation: "",
					}},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "channel",
				},
			},
			want: "name:channel:2",
		},
		{
			name: "Annotations is nil",
			template: &v1beta2.ModuleTemplate{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:       "name",
					Generation: 3,
				},
				Spec: v1beta2.ModuleTemplateSpec{
					Channel: "channel",
				},
			},
			want: "name:channel:3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := cache.GenerateDescriptorCacheKey(tc.template)
			assert.Equal(t, tc.want, got)
		})
	}
}
