package skrclient

import (
	"context"
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
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
)

const (
	apis = "/apis"
	api  = "/api"
)

type Client interface {
	skrresources.ResourceInfoConverter

	client.Client
}

type MappingResolver func(obj machineryruntime.Object, mapper meta.RESTMapper) (*meta.RESTMapping,
	error,
)

type ResourceInfoClientResolver func(obj *unstructured.Unstructured,
	skrClient *SKRClient,
	mapping *meta.RESTMapping,
) (resource.RESTClient,
	error,
)

type Service struct {
	qps                  float32
	burst                int
	accessManagerService AccessManagerService
}

type AccessManagerService interface {
	GetAccessRestConfigByKyma(ctx context.Context, kymaName string) (*rest.Config, error)
}

func NewService(qps float32, burst int, accessManagerService AccessManagerService) *Service {
	return &Service{
		qps:                  qps,
		burst:                burst,
		accessManagerService: accessManagerService,
	}
}

// SKRClient serves as a single-minded client interface that combines
// all kubernetes Client APIs (Kubernetes, Client-Go) under the hood.
// It offers a simple initialization lifecycle during creation, but delegates all
// heavy-duty work to deferred discovery logic and a single http client
// as well as a client cache to support GV-based clients.
type SKRClient struct {
	// controller runtime client
	client.Client

	// the original config used for all clients
	config *rest.Config

	// expander for GVK and REST expansion from discovery client
	discoveryShortcutExpander meta.RESTMapper
	// GVK based structured Client Cache
	structuredSyncLock        sync.Mutex
	structuredRESTClientCache map[string]resource.RESTClient

	// GVK based unstructured Client Cache
	unstructuredSyncLock        sync.Mutex
	unstructuredRESTClientCache map[string]resource.RESTClient

	mappingResolver            MappingResolver
	resourceInfoClientResolver ResourceInfoClientResolver
}

func (s *Service) ResolveClient(ctx context.Context, manifest *v1beta2.Manifest) (*SKRClient, error) {
	kymaName, err := manifest.GetKymaName()
	if err != nil {
		return nil, fmt.Errorf("failed to get kyma owner label: %w", err)
	}

	config, err := s.accessManagerService.GetAccessRestConfigByKyma(ctx, kymaName)
	if err != nil {
		return nil, err
	}

	config.QPS = s.qps
	config.Burst = s.burst

	if err := setKubernetesDefaults(config); err != nil {
		return nil, err
	}

	// Required to prevent memory leak by avoiding caching in transport.tlsTransportCache. Service are cached anyways.
	config.Proxy = http.ProxyFromEnvironment

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initiliaze DiscoveryClient: %w", err)
	}

	cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)

	discoveryRESTMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)
	discoveryShortcutExpander := restmapper.NewShortcutExpander(discoveryRESTMapper, cachedDiscoveryClient, nil)

	runtimeClient, err := newClientProxy(config, discoveryShortcutExpander)
	if err != nil {
		return nil, err
	}

	clients := &SKRClient{
		config:                      config,
		discoveryShortcutExpander:   discoveryShortcutExpander,
		structuredRESTClientCache:   map[string]resource.RESTClient{},
		unstructuredRESTClientCache: map[string]resource.RESTClient{},
		Client:                      runtimeClient,
		mappingResolver:             getResourceMapping,
		resourceInfoClientResolver:  getResourceInfoClient,
	}

	return clients, nil
}

func (s *SKRClient) SetMappingResolver(resolver MappingResolver) {
	s.mappingResolver = resolver
}

func (s *SKRClient) SetResourceInfoClientResolver(resolver ResourceInfoClientResolver) {
	s.resourceInfoClientResolver = resolver
}

func (s *SKRClient) ResourceInfo(obj *unstructured.Unstructured) (*resource.Info, error) {
	mapping, err := s.mappingResolver(obj, s.discoveryShortcutExpander)
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
	client *SKRClient,
	mapping *meta.RESTMapping,
) (resource.RESTClient,
	error,
) {
	var clnt resource.RESTClient
	var err error
	if client.Scheme().IsGroupRegistered(mapping.GroupVersionKind.Group) {
		clnt, err = client.ClientForMapping(mapping)
		if err != nil {
			return nil, err
		}
	} else {
		clnt, err = client.UnstructuredClientForMapping(mapping)
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

func getResourceMapping(obj machineryruntime.Object, mapper meta.RESTMapper) (*meta.RESTMapping,
	error,
) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if gvk.Empty() {
		return mapping, nil
	}

	if meta.IsNoMatchError(err) {
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
