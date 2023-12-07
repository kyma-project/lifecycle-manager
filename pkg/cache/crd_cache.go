package cache

import (
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

//nolint:gochecknoglobals // in-memory cache used for CRDs
var crdCache = sync.Map{}

func GetCachedCRD(key string) (apiextensionsv1.CustomResourceDefinition, bool) {
	value, ok := crdCache.Load(key)
	if !ok {
		return apiextensionsv1.CustomResourceDefinition{}, false
	}
	crd, ok := value.(apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return apiextensionsv1.CustomResourceDefinition{}, false
	}

	return crd, true
}

func SetCRDInCache(key string, value apiextensionsv1.CustomResourceDefinition) {
	crdCache.Store(key, value)
}
