package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRandomTTL(t *testing.T) {
	ttl := getRandomTTL()
	assert.GreaterOrEqual(t, ttl.Seconds(), float64(23*60*60), "TTL should be at least 23 hours")
	assert.LessOrEqual(t, ttl.Seconds(), float64(25*60*60), "TTL should be at most 25 hours")
}
