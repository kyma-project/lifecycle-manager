package remote_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"
)

func TestNewClientCache(t *testing.T) {
	cache := remote.NewClientCache(nil, nil)

	assert.NotNil(t, cache)
}

func TestGet_WhenCalledOnEmptyCache_ReturnsNil(t *testing.T) {
	cache := remote.NewClientCache(nil, nil)

	result := cache.Get(client.ObjectKey{})

	assert.Nil(t, result)
}

func TestSet_WhenCalledWithEmptyValue_GetReturnsNil(t *testing.T) {
	cache := remote.NewClientCache(nil, nil)

	cache.Set(client.ObjectKey{}, nil)

	result := cache.Get(client.ObjectKey{})

	assert.Nil(t, result)
}

func TestDelete_WhenCalled_DeletesCachedClient(t *testing.T) {
	cache := remote.NewClientCache(nil, nil)
	key := client.ObjectKey{
		Namespace: "key-namespace",
		Name:      "key",
	}
	cache.Set(key, &remote.ConfigAndClient{})

	result := cache.Get(key)
	assert.NotNil(t, result)

	cache.Del(key)
	result = cache.Get(key)

	assert.Nil(t, result)
}
