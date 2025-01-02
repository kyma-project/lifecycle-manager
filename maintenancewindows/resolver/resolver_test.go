package resolver

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMaintenancePolicyPool(t *testing.T) {
	err := os.Setenv(policyPathENV, "./testdata")
	assert.NoError(t, err)

	pool, err := GetMaintenancePolicyPool()
	assert.NoError(t, err)

	assert.Equal(t, 2, len(pool))
	assert.Contains(t, pool, "ruleset-1.json")
	assert.Contains(t, pool, "ruleset-2.json")

	data1 := pool["ruleset-1.json"]
	data2 := pool["ruleset-2.json"]
	assert.NotNil(t, data1)
	assert.NotNil(t, data2)
}
