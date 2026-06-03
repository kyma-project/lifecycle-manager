//nolint:testpackage // testing package internals
package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_Basic(t *testing.T) {
	svc := NewService()

	require.Equal(t, 0, svc.Size(), "expected size 0")

	svc.AddClient("a", nil)
	require.Equal(t, 1, svc.Size(), "expected size 1 after add")

	require.Nil(t, svc.GetClient("a"), "expected nil client value")

	svc.AddClient("b", nil)
	require.Equal(t, 2, svc.Size(), "expected size 2 after second add")

	svc.DeleteClient("a")
	require.Equal(t, 1, svc.Size(), "expected size 1 after delete")

	require.Nil(t, svc.GetClient("a"), "expected nil for deleted key")
}

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
