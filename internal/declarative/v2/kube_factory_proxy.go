package v2

import (
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/openapi"
)

// OpenAPISchema returns metadata and structural information about
// Kubernetes object definitions.
func (s *SingletonClients) OpenAPISchema() (openapi.Resources, error) {
	parsedMetadata, err := s.openAPIParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema metadata: %w", err)
	}
	return parsedMetadata, nil
}

// OpenAPIGetter returns a getter for the openapi schema document.
func (s *SingletonClients) OpenAPIGetter() *openapi.CachedOpenAPIGetter {
	return s.openAPIGetter
}

// UnstructuredClientForMapping returns a RESTClient for working with Unstructured objects.
func (s *SingletonClients) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	s.unstructuredSyncLock.Lock()
	defer s.unstructuredSyncLock.Unlock()
	key := s.clientCacheKeyForMapping(mapping)
	client, found := s.unstructuredRESTClientCache[key]

	if found {
		return client, nil
	}

	cfg := rest.CopyConfig(s.config)
	cfg.APIPath = apis
	if mapping.GroupVersionKind.Group == apicorev1.GroupName {
		cfg.APIPath = api
	}
	gv := mapping.GroupVersionKind.GroupVersion()
	cfg.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	cfg.GroupVersion = &gv

	client, err := rest.RESTClientForConfigAndClient(cfg, s.httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create httpClient config: %w", err)
	}
	s.unstructuredRESTClientCache[key] = client
	return client, nil
}

// ClientForMapping returns a RESTClient for working with the specified RESTMapping or an error. This is intended
// for working with arbitrary resources and is not guaranteed to point to a Kubernetes APIServer.
func (s *SingletonClients) ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	s.structuredSyncLock.Lock()
	defer s.structuredSyncLock.Unlock()
	key := s.clientCacheKeyForMapping(mapping)
	client, found := s.structuredRESTClientCache[key]

	if found {
		return client, nil
	}

	cfg := rest.CopyConfig(s.config)
	gvk := mapping.GroupVersionKind
	switch gvk.Group {
	case apicorev1.GroupName:
		cfg.APIPath = api
	default:
		cfg.APIPath = apis
	}
	gv := gvk.GroupVersion()
	cfg.GroupVersion = &gv

	client, err := rest.RESTClientForConfigAndClient(cfg, s.httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create httpClient config: %w", err)
	}

	s.structuredRESTClientCache[key] = client
	return client, nil
}
