package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	descriptorcache "github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestGenerateDescriptorKey(t *testing.T) {
	t.Run("should generate correct descriptor cache key", func(t *testing.T) {
		moduleName := "name"
		moduleVersion := "1.0.0"
		expected := "name:1.0.0"

		ocmId := testutils.MustNewComponentId(moduleName, moduleVersion)
		res := descriptorcache.GenerateDescriptorKey(*ocmId)
		assert.Equal(t, expected, string(res))
	})
}
