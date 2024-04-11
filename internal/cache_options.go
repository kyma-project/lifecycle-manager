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
	CacheOptions cache.Options
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
					"kcp-system":   {},
					"istio-system": {},
					"kyma-system":  {},
				},
			},
			&v1beta2.Kyma{}: {
				Namespaces: map[string]cache.Config{
					"kyma-system": {},
					"kcp-system":  {},
				},
			},
			&v1beta2.ModuleTemplate{}: {
				Namespaces: map[string]cache.Config{
					"kyma-system": {},
					"kcp-system":  {},
				},
			},
			&v1beta2.Manifest{}: {
				Namespaces: map[string]cache.Config{
					"kcp-system": {},
				},
			},
			&v1beta2.Watcher{}: {
				Namespaces: map[string]cache.Config{
					"kcp-system": {},
				},
			},
			&certmanagerv1.Issuer{}: {
				Namespaces: map[string]cache.Config{
					"kcp-system":   {},
					"istio-system": {},
				},
			},
			&certmanagerv1.Certificate{}: {
				Namespaces: map[string]cache.Config{
					"kcp-system":   {},
					"istio-system": {},
				},
			},
		},
	}
}

func GetCacheOptions(isKymaManaged bool) cache.Options {
	if isKymaManaged {
		options := &KcpCacheOptions{}
		return options.GetCacheOptions()
	}

	options := &DefaultCacheOptions{}
	return options.GetCacheOptions()
}
