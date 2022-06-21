package template

import "github.com/pkg/errors"

type Cache interface {
	Get(key interface{}) (*LookupResult, error)
	Put(key interface{}, value *LookupResult) error
	Delete(key interface{}) error
}

func NewInMemoryCache() Cache {
	return &inMemoryCache{backingCache: map[string]*LookupResult{}}
}

type inMemoryCache struct {
	backingCache LookupResultsByName
}

func (i *inMemoryCache) Get(key interface{}) (*LookupResult, error) {
	k, ok := key.(string)
	if !ok {
		return nil, errors.Errorf("template with key %s does not exist", k)
	}
	return i.backingCache[k], nil
}

func (i *inMemoryCache) Put(key interface{}, value *LookupResult) error {
	k, ok := key.(string)
	if !ok {
		return errors.Errorf("template with key %s could not be saved to cache", k)
	}
	i.backingCache[k] = value
	return nil
}

func (i *inMemoryCache) Delete(key interface{}) error {
	k, ok := key.(string)
	if !ok {
		return errors.Errorf("template with key %s could not be deleted", k)
	}
	i.backingCache[k] = nil
	return nil
}
