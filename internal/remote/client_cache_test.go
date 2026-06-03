//nolint:testpackage // testing package internals
package remote

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewClientCache(t *testing.T) {
	cache := NewClientCache()

	assert.NotNil(t, cache)
	assert.Equal(t, 0, cache.Size())
}

func TestAddGetClientCache(t *testing.T) {
	cache := NewClientCache()
	key := client.ObjectKey{}
	value := &testClient{}

	cache.Add(key, value)

	result := cache.Get(key)
	assert.NotNil(t, result)
	assert.Equal(t, value, result)
}

func TestDeleteClientCache(t *testing.T) {
	cache := NewClientCache()
	key := client.ObjectKey{}
	value := &testClient{}

	cache.Add(key, value)

	cache.Delete(key)

	result := cache.Get(key)
	assert.Nil(t, result)
}

func TestSizeClientCache(t *testing.T) {
	cache := NewClientCache()
	key1, key2 := client.ObjectKey{
		Namespace: "namespace-1",
		Name:      "name-1",
	}, client.ObjectKey{
		Namespace: "namespace-2",
		Name:      "name-2",
	}
	value1 := &testClient{name: "client-1"}
	value2 := &testClient{name: "client-2"}

	cache.Add(key1, value1)
	cache.Add(key2, value2)

	assert.Equal(t, 2, cache.Size())

	cache.Delete(key1)

	assert.Equal(t, 1, cache.Size())

	result := cache.Get(key2)
	assert.IsType(t, &testClient{}, result)
}

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

func (c *testClient) Config() *rest.Config { return nil }
