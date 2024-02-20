package provider

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/pkg/object/cache"
)

var ErrInvalidProviderArgument = errors.New("objectProvider must not be nil")

type CachedObjectProvider struct {
	objectProvider *ObjectProvider
	objectCache    *cache.ObjectCache
}

func NewCachedObjectProvider(objectProvider *ObjectProvider, objectCache *cache.ObjectCache) (*CachedObjectProvider, error) {
	if objectProvider == nil {
		return nil, ErrInvalidProviderArgument
	}

	if objectCache != nil {
		return &CachedObjectProvider{
			objectProvider: objectProvider,
			objectCache:    objectCache,
		}, nil
	}

	return &CachedObjectProvider{
		objectProvider: objectProvider,
		objectCache:    cache.NewObjectCache(),
	}, nil
}

func (p CachedObjectProvider) Get(ctx context.Context) (*unstructured.Unstructured, error) {
	key := cache.GenerateObjectKey(p.objectProvider.Name, p.objectProvider.GroupVersionKind)

	object := p.objectCache.Get(key)
	if object != nil {
		return object, nil
	}

	object, err := p.objectProvider.Get(ctx)
	if err != nil {
		return nil, err
	}

	p.objectCache.Set(key, object)

	return object, nil
}
