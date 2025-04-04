package internal

import (
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type CacheOptions interface {
	GetCacheOptions() cache.Options
}

type DefaultCacheOptions struct {
	CacheOptions cache.Options
}

type KcpCacheOptions struct {
	CacheOptions          cache.Options
	istioNamespace        string
	kcpNamespace          string
	certManagementObjects []client.Object
}

func (c *DefaultCacheOptions) GetCacheOptions() cache.Options {
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&apicorev1.Secret{}: {
				Label: k8slabels.Everything(),
			},
		},
	}
}

func (c *KcpCacheOptions) GetCacheOptions() cache.Options {
	options := cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&apicorev1.Secret{}: {
				Label: k8slabels.Everything(),
				Namespaces: map[string]cache.Config{
					c.kcpNamespace:   {},
					c.istioNamespace: {},
				},
			},
			&v1beta2.Kyma{}: {
				Namespaces: map[string]cache.Config{
					c.kcpNamespace: {},
				},
			},
			&v1beta2.ModuleTemplate{}: {
				Namespaces: map[string]cache.Config{
					c.kcpNamespace: {},
				},
			},
			&v1beta2.ModuleReleaseMeta{}: {
				Namespaces: map[string]cache.Config{
					c.kcpNamespace: {},
				},
			},
			&v1beta2.Manifest{}: {
				Namespaces: map[string]cache.Config{
					c.kcpNamespace: {},
				},
			},
			&v1beta2.Watcher{}: {
				Namespaces: map[string]cache.Config{
					c.kcpNamespace: {},
				},
			},
		},
	}

	for _, certManagementObject := range c.certManagementObjects {
		options.ByObject[certManagementObject] = cache.ByObject{
			Namespaces: map[string]cache.Config{
				c.kcpNamespace:   {},
				c.istioNamespace: {},
			},
		}
	}

	return options
}

func GetCacheOptions(isKymaManaged bool,
	istioNamespace,
	kcpNamespace string,
	certManagementObjects []client.Object,
) cache.Options {
	if isKymaManaged {
		options := &KcpCacheOptions{
			istioNamespace:        istioNamespace,
			kcpNamespace:          kcpNamespace,
			certManagementObjects: certManagementObjects,
		}
		return options.GetCacheOptions()
	}

	options := &DefaultCacheOptions{}
	return options.GetCacheOptions()
}
