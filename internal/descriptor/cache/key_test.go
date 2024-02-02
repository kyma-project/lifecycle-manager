package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestGenerateDescriptorCacheKey(t *testing.T) {
	testCases := []struct {
		name     string
		template *v1beta2.ModuleTemplate
		want     cache.DescriptorKey
	}{
		{
			name: "Annotations is not nil and valid semver",
			template: builder.NewModuleTemplateBuilder().
				WithName("name").
				WithAnnotation(shared.ModuleVersionAnnotation, "1.0.0").
				WithChannel("channel").
				Build(),
			want: "name:channel:1.0.0",
		},
		{
			name: "Annotations is not nil but invalid semver",
			template: builder.NewModuleTemplateBuilder().
				WithName("name").
				WithGeneration(1).
				WithAnnotation(shared.ModuleVersionAnnotation, "not-semver").
				WithChannel("channel").
				Build(),
			want: "name:channel:1",
		},
		{
			name: "Annotations is not nil but module version is empty",
			template: builder.NewModuleTemplateBuilder().
				WithName("name").
				WithGeneration(2).
				WithAnnotation(shared.ModuleVersionAnnotation, "").
				WithChannel("channel").
				Build(),
			want: "name:channel:2",
		},
		{
			name: "Annotations is nil",
			template: builder.NewModuleTemplateBuilder().
				WithName("name").
				WithGeneration(3).
				WithChannel("channel").
				Build(),
			want: "name:channel:3",
		},
	}

	for i := range testCases {
		i := i
		t.Run(testCases[i].name, func(t *testing.T) {
			got := cache.GenerateDescriptorKey(testCases[i].template)
			assert.Equalf(t, testCases[i].want, got,
				"GetComponentDescriptorCacheKey() = %v, want %v", got, testCases[i].want)
		})
	}
}
