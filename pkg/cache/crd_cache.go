package cache

import (
	"sync"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

//nolint:gochecknoglobals
var crdCache = sync.Map{}

func GetCachedCRD(key string) (v1.CustomResourceDefinition, bool) {
	value, ok := crdCache.Load(key)
	if !ok {
		return v1.CustomResourceDefinition{}, false
	}
	crd, ok := value.(v1.CustomResourceDefinition)
	if !ok {
		return v1.CustomResourceDefinition{}, false
	}

	return crd, true
}

func SetCRDInCache(key string, value v1.CustomResourceDefinition) {
	crdCache.Store(key, value)
}
