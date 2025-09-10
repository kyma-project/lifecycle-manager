package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

func TestGenerateDescriptorCacheKey(t *testing.T) {
	testCases := []struct {
		name          string
		moduleName    string
		moduleVersion string
		want          string
	}{
		{
			name:          "with valid module name and version",
			moduleName:    "name",
			moduleVersion: "1.0.0",
			want:          "name:1.0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ocmi := ocmidentity.MustNew(tc.moduleName, tc.moduleVersion)
			got := cache.GenerateDescriptorKey(*ocmi)
			assert.Equal(t, tc.want, string(got))
		})
	}
}
