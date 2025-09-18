package remote_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/remote"
)

func TestNewClientCache(t *testing.T) {
	cache := remote.NewClientCache()

	assert.NotNil(t, cache)
	assert.Equal(t, 0, cache.Size())
}

func TestAddGetClientCache(t *testing.T) {
	cache := remote.NewClientCache()
	key := client.ObjectKey{}
	value := &TestClient{}

	cache.Add(key, value)

	result := cache.Get(key)
	assert.NotNil(t, result)
	assert.Equal(t, value, result)
}

func TestDeleteClientCache(t *testing.T) {
	cache := remote.NewClientCache()
	key := client.ObjectKey{}
	value := &TestClient{}

	cache.Add(key, value)

	cache.Delete(key)

	result := cache.Get(key)
	assert.Nil(t, result)
}

func TestSizeClientCache(t *testing.T) {
	cache := remote.NewClientCache()
	key1, key2 := client.ObjectKey{
		Namespace: "namespace-1",
		Name:      "name-1",
	}, client.ObjectKey{
		Namespace: "namespace-2",
		Name:      "name-2",
	}
	value1 := &TestClient{name: "client-1"}
	value2 := &TestClient{name: "client-2"}

	cache.Add(key1, value1)
	cache.Add(key2, value2)

	assert.Equal(t, 2, cache.Size())

	cache.Delete(key1)

	assert.Equal(t, 1, cache.Size())

	result := cache.Get(key2)
	assert.IsType(t, &TestClient{}, result)
}

type TestClient struct {
	client.Client

	name string
}

func (c *TestClient) Config() *rest.Config { return nil }
