package resolver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/maintenancewindows/resolver"
)

func TestGetMaintenancePolicyPool(t *testing.T) {
	t.Setenv(resolver.PolicyPathENV, "./testdata")

	pool, err := resolver.GetMaintenancePolicyPool()
	require.NoError(t, err)

	assert.Len(t, pool, 2)
	assert.Contains(t, pool, "ruleset-1.json")
	assert.Contains(t, pool, "ruleset-2.json")

	data1 := pool["ruleset-1.json"]
	data2 := pool["ruleset-2.json"]
	assert.NotNil(t, data1)
	assert.NotNil(t, data2)
}
