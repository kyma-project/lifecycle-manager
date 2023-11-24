package v2

import (
	"fmt"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
)

const (
	apis = "/apis"
	api  = "/api"
)

// SingletonClients serves as a single-minded client interface that combines
// all kubernetes Client APIs (Kubernetes, Client-Go) under the hood.
// It offers a simple initialization lifecycle during creation, but delegates all
// heavy-duty work to deferred discovery logic and a single http client
// as well as a client cache to support GV-based clients.
type SingletonClients struct {
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
}

func NewSingletonClients(info *ClusterInfo) (*SingletonClients, error) {
	if err := setKubernetesDefaults(info.Config); err != nil {
		return nil, err
	}

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
	discoveryShortcutExpander := restmapper.NewShortcutExpander(discoveryRESTMapper, cachedDiscoveryClient)

	// Create target cluster client only if not passed.
	// Clients should be passed only in two cases:
	// 1. Single cluster mode is enabled.
	// Since such clients are similar to the root client instance.
	// 2. Client instance is explicitly passed from the library interface
	runtimeClient := info.Client
	if info.Client == nil {
		// For all other cases where a client instance is not passed, create a client proxy.
		runtimeClient, err = NewClientProxy(info.Config, discoveryShortcutExpander)
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

	clients := &SingletonClients{
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
	}

	return clients, nil
}

func (s *SingletonClients) clientCacheKeyForMapping(mapping *meta.RESTMapping) string {
	return fmt.Sprintf(
		"%s+%s:%s",
		mapping.Resource.String(), mapping.GroupVersionKind.String(), mapping.Scope.Name(),
	)
}

func (s *SingletonClients) ResourceInfo(obj *unstructured.Unstructured, retryOnNoMatch bool) (*resource.Info, error) {
	mapping, err := getResourceMapping(obj, s.discoveryShortcutExpander, retryOnNoMatch)
	if err != nil {
		return nil, err
	}
	info := &resource.Info{}

	var clnt resource.RESTClient
	if s.Scheme().IsGroupRegistered(mapping.GroupVersionKind.Group) {
		clnt, err = s.ClientForMapping(mapping)
		if err != nil {
			return nil, err
		}
	} else {
		clnt, err = s.UnstructuredClientForMapping(mapping)
		if err != nil {
			return nil, err
		}
		obj.SetGroupVersionKind(mapping.GroupVersionKind)
	}

	info.Client = clnt
	info.Mapping = mapping
	info.Namespace = obj.GetNamespace()
	info.Name = obj.GetName()
	info.Object = obj
	info.ResourceVersion = obj.GetResourceVersion()
	return info, nil
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
