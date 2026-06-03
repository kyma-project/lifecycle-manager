//nolint:testpackage // testing package internals to verify TTL eviction behaviour
package remote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestClientCache_TTLExpiresAfterRepeatedGets(t *testing.T) {
	cache := NewClientCache()
	key := client.ObjectKey{Namespace: "ns", Name: "kyma"}
	value := &testClient{name: "client"}

	const shortTTL = 50 * time.Millisecond
	cache.internal.Set(key, value, shortTTL)

	// Multiple Gets must NOT extend the expiry (WithDisableTouchOnHit).
	for range 5 {
		assert.NotNil(t, cache.Get(key), "expected client to be present before TTL expiry")
	}

	time.Sleep(shortTTL + 10*time.Millisecond)

	assert.Nil(t, cache.Get(key), "expected client to be evicted after TTL expiry")
}

type testClient struct {
	client.Client
	name string
}
