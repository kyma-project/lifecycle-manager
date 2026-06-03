//nolint:testpackage // testing package internals to verify TTL eviction behaviour
package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_TTLExpiresAfterRepeatedGets(t *testing.T) {
	svc := NewService()

	const shortTTL = 50 * time.Millisecond
	svc.internal.Set("skr-a", nil, shortTTL)

	// Multiple GetClient calls must NOT extend the expiry (WithDisableTouchOnHit).
	for range 5 {
		_ = svc.GetClient("skr-a")
	}

	time.Sleep(shortTTL + 10*time.Millisecond)

	require.Nil(t, svc.GetClient("skr-a"), "expected client to be evicted after TTL expiry")
	require.Equal(t, 0, svc.Size(), "expected cache to be empty after TTL expiry")
}
