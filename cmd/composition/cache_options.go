package composition

import (
	"os"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common"
	"github.com/kyma-project/lifecycle-manager/internal/repository/certificate/certmanager"
	"github.com/kyma-project/lifecycle-manager/internal/repository/certificate/gardener"
)

func ComposeCacheOptions(isKymaManaged bool,
	istioNamespace string,
	kcpNamespace string,
	certificateManagement string, setupLog logr.Logger,
) cache.Options {
	if isKymaManaged {
		options := &KcpCacheOptions{
			istioNamespace:             istioNamespace,
			kcpNamespace:               kcpNamespace,
			certManagementCacheObjects: getCertManagementCacheObjects(certificateManagement, setupLog),
		}
		return options.GetCacheOptions()
	}

	options := &DefaultCacheOptions{}
	return options.GetCacheOptions()
}

type CacheOptions interface {
	GetCacheOptions() cache.Options
}

type DefaultCacheOptions struct {
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

type KcpCacheOptions struct {
	CacheOptions               cache.Options
	istioNamespace             string
	kcpNamespace               string
	certManagementCacheObjects []client.Object
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

	for _, certManagementObject := range c.certManagementCacheObjects {
		options.ByObject[certManagementObject] = cache.ByObject{
			Namespaces: map[string]cache.Config{
				c.kcpNamespace:   {},
				c.istioNamespace: {},
			},
		}
	}

	return options
}

func getCertManagementCacheObjects(certificateManagement string, setupLog logr.Logger) []client.Object {
	cacheObjects, ok := map[string][]client.Object{
		certmanagerv1.SchemeGroupVersion.String(): certmanager.GetCacheObjects(),
		gcertv1alpha1.SchemeGroupVersion.String(): gardener.GetCacheObjects(),
	}[certificateManagement]

	if !ok {
		setupLog.Error(common.ErrUnsupportedCertificateManagementSystem,
			"unable to get cache options for certificate management")
		os.Exit(bootstrapFailedExitCode)
	}

	return cacheObjects
}
