package skrclient

import (
	"fmt"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/util/openapi"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
)

const (
	apis = "/apis"
	api  = "/api"
)

type Client interface {
	resource.RESTClientGetter
	skrresources.ResourceInfoConverter

	client.Client
}

type MappingResolver func(obj machineryruntime.Object, mapper meta.RESTMapper, retryOnNoMatch bool) (*meta.RESTMapping,
	error,
)

type ResourceInfoClientResolver func(obj *unstructured.Unstructured,
	s *Service,
	mapping *meta.RESTMapping,
) (resource.RESTClient,
	error,
)

// Service serves as a single-minded client interface that combines
// all kubernetes Client APIs (Kubernetes, Client-Go) under the hood.
// It offers a simple initialization lifecycle during creation, but delegates all
// heavy-duty work to deferred discovery logic and a single http client
// as well as a client cache to support GV-based clients.
type Service struct {
	httpClient *http.Client

	// controller runtime client
	client.Client

	// the original config used for all clients
	config *rest.Config

	// discovery client, used for dynamic clients and GVK discovery
	discoveryClient discovery.CachedDiscoveryInterface
	// expander for GVK and REST expansion from discovery client
	discoveryShortcutExpander meta.RESTMapper

	// kubernetes client
	kubernetesClient *kubernetes.Clientset
	dynamicClient    *dynamic.DynamicClient

	// OpenAPI document parser singleton
	openAPIParser *openapi.CachedOpenAPIParser

	// OpenAPI document getter singleton
	openAPIGetter *openapi.CachedOpenAPIGetter

	// GVK based structured Client Cache
	structuredSyncLock        sync.Mutex
	structuredRESTClientCache map[string]resource.RESTClient

	// GVK based unstructured Client Cache
	unstructuredSyncLock        sync.Mutex
	unstructuredRESTClientCache map[string]resource.RESTClient

	mappingResolver            MappingResolver
	resourceInfoClientResolver ResourceInfoClientResolver
}

func NewService(info *ClusterInfo) (*Service, error) {
	if err := setKubernetesDefaults(info.Config); err != nil {
		return nil, err
	}

	// Required to prevent memory leak by avoiding caching in transport.tlsTransportCache. Service are cached anyways.
	info.Config.Proxy = http.ProxyFromEnvironment

	httpClient, err := rest.HTTPClientFor(info.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to initiliaze httpClient: %w", err)
	}

	discoveryConfig := *info.Config
	discoveryConfig.Burst = 200
	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(&discoveryConfig, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initiliaze DiscoveryClient: %w", err)
	}
	cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)
	discoveryRESTMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)
	discoveryShortcutExpander := restmapper.NewShortcutExpander(discoveryRESTMapper, cachedDiscoveryClient, nil)

	// Create target cluster client only if not passed.
	// Clients should be passed only in two cases:
	// 1. Single cluster mode is enabled.
	// Since such clients are similar to the root client instance.
	// 2. Client instance is explicitly passed from the library interface
	runtimeClient := info.Client
	if info.Client == nil {
		// For all other cases where a client instance is not passed, create a client proxy.
		runtimeClient, err = newClientProxy(info.Config, discoveryShortcutExpander)
		if err != nil {
			return nil, err
		}
	}

	kubernetesClient, err := kubernetes.NewForConfigAndClient(info.Config, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialiaze k8s-client: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfigAndClient(info.Config, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialiaze dynamic-client: %w", err)
	}

	openAPIGetter := openapi.NewOpenAPIGetter(cachedDiscoveryClient)

	clients := &Service{
		httpClient:                  httpClient,
		config:                      info.Config,
		discoveryClient:             cachedDiscoveryClient,
		discoveryShortcutExpander:   discoveryShortcutExpander,
		kubernetesClient:            kubernetesClient,
		dynamicClient:               dynamicClient,
		openAPIGetter:               openAPIGetter,
		openAPIParser:               openapi.NewOpenAPIParser(openAPIGetter),
		structuredRESTClientCache:   map[string]resource.RESTClient{},
		unstructuredRESTClientCache: map[string]resource.RESTClient{},
		Client:                      runtimeClient,
		mappingResolver:             getResourceMapping,
		resourceInfoClientResolver:  getResourceInfoClient,
	}

	return clients, nil
}

func (s *Service) SetMappingResolver(resolver MappingResolver) {
	s.mappingResolver = resolver
}

func (s *Service) SetResourceInfoClientResolver(resolver ResourceInfoClientResolver) {
	s.resourceInfoClientResolver = resolver
}

func (s *Service) ResourceInfo(obj *unstructured.Unstructured, retryOnNoMatch bool) (*resource.Info, error) {
	mapping, err := s.mappingResolver(obj, s.discoveryShortcutExpander, retryOnNoMatch)
	if err != nil {
		return nil, err
	}

	clnt, err := s.resourceInfoClientResolver(obj, s, mapping)
	if err != nil {
		return nil, err
	}

	info := &resource.Info{}
	info.Client = clnt
	info.Mapping = mapping
	info.Namespace = obj.GetNamespace()
	info.Name = obj.GetName()
	info.Object = obj
	info.ResourceVersion = obj.GetResourceVersion()
	return info, nil
}

func getResourceInfoClient(obj *unstructured.Unstructured,
	service *Service,
	mapping *meta.RESTMapping,
) (resource.RESTClient,
	error,
) {
	var clnt resource.RESTClient
	var err error
	if service.Scheme().IsGroupRegistered(mapping.GroupVersionKind.Group) {
		clnt, err = service.ClientForMapping(mapping)
		if err != nil {
			return nil, err
		}
	} else {
		clnt, err = service.UnstructuredClientForMapping(mapping)
		if err != nil {
			return nil, err
		}
		obj.SetGroupVersionKind(mapping.GroupVersionKind)
	}
	return clnt, nil
}

func setKubernetesDefaults(config *rest.Config) error {
	config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}

	if config.APIPath == "" {
		config.APIPath = "/api"
	}
	if config.NegotiatedSerializer == nil {
		// This codec factory ensures the resources are not converted. Therefore, resources
		// will not be round-tripped through internal versions. Defaulting does not happen
		// on the client.
		config.NegotiatedSerializer = k8sclientscheme.Codecs.WithoutConversion()
	}
	err := rest.SetKubernetesDefaults(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes default config: %w", err)
	}
	return nil
}

func getResourceMapping(obj machineryruntime.Object, mapper meta.RESTMapper, retryOnNoMatch bool) (*meta.RESTMapping,
	error,
) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if gvk.Empty() {
		return mapping, nil
	}

	if retryOnNoMatch && meta.IsNoMatchError(err) {
		// reset mapper if a NoMatchError is reported on the first call
		meta.MaybeResetRESTMapper(mapper)
		// return second call after reset
		mapping, err = mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}

	if err != nil {
		return nil, fmt.Errorf("failed rest mapping [%v, %v]: %w", gvk.GroupKind(), gvk.Version, err)
	}

	return mapping, nil
}
