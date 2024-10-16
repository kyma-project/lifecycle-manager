package internal

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
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
	CacheOptions    cache.Options
	istioNamespace  string
	kcpNamespace    string
	remoteNamespace string
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
	return cache.Options{
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
					c.remoteNamespace: {},
					c.kcpNamespace:    {},
				},
			},
			&v1beta2.ModuleTemplate{}: {
				Namespaces: map[string]cache.Config{
					c.remoteNamespace: {},
					c.kcpNamespace:    {},
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
			&certmanagerv1.Issuer{}: {
				Namespaces: map[string]cache.Config{
					c.kcpNamespace:   {},
					c.istioNamespace: {},
				},
			},
			&certmanagerv1.Certificate{}: {
				Namespaces: map[string]cache.Config{
					c.kcpNamespace:   {},
					c.istioNamespace: {},
				},
			},
		},
	}
}

func GetCacheOptions(isKymaManaged bool, istioNamespace, kcpNamespace, remoteNamespace string) cache.Options {
	if isKymaManaged {
		options := &KcpCacheOptions{
			istioNamespace:  istioNamespace,
			kcpNamespace:    kcpNamespace,
			remoteNamespace: remoteNamespace,
		}
		return options.GetCacheOptions()
	}

	options := &DefaultCacheOptions{}
	return options.GetCacheOptions()
}
