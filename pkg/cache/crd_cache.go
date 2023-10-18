package cache

import (
	"sync"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

//nolint:gochecknoglobals
var crdCache = sync.Map{}

func GetCachedCRD(key string) (apiextensions.CustomResourceDefinition, bool) {
	value, ok := crdCache.Load(key)
	if !ok {
		return apiextensions.CustomResourceDefinition{}, false
	}
	crd, ok := value.(apiextensions.CustomResourceDefinition)
	if !ok {
		return apiextensions.CustomResourceDefinition{}, false
	}

	return crd, true
}

func SetCRDInCache(key string, value apiextensions.CustomResourceDefinition) {
	crdCache.Store(key, value)
}
