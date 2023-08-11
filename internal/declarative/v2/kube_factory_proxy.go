package v2

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/openapi"
	"k8s.io/kubectl/pkg/validation"
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
func (s *SingletonClients) OpenAPIGetter() discovery.OpenAPISchemaInterface {
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
	if mapping.GroupVersionKind.Group == corev1.GroupName {
		cfg.APIPath = api
	}
	gv := mapping.GroupVersionKind.GroupVersion()
	cfg.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	cfg.GroupVersion = &gv

	var err error
	client, err = rest.RESTClientForConfigAndClient(cfg, s.httpClient)
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
	case corev1.GroupName:
		cfg.APIPath = api
	default:
		cfg.APIPath = apis
	}
	gv := gvk.GroupVersion()
	cfg.GroupVersion = &gv

	var err error
	client, err = rest.RESTClientForConfigAndClient(cfg, s.httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create httpClient config: %w", err)
	}

	s.structuredRESTClientCache[key] = client
	return client, nil
}

// KubernetesClientSet gives you back an external clientset.
func (s *SingletonClients) KubernetesClientSet() (*kubernetes.Clientset, error) {
	return s.kubernetesClient, nil
}

// DynamicClient returns a dynamic client ready for use.
func (s *SingletonClients) DynamicClient() (dynamic.Interface, error) {
	return s.dynamicClient, nil
}

// NewBuilder returns a new resource builder for structured api objects.
func (s *SingletonClients) NewBuilder() *resource.Builder {
	return resource.NewBuilder(s)
}

// RESTClient returns a RESTClient for accessing Kubernetes resources or an error.
func (s *SingletonClients) RESTClient() (*rest.RESTClient, error) {
	restClient, err := rest.RESTClientForConfigAndClient(s.config, s.httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize RestCliient for k8s api access: %w", err)
	}
	return restClient, nil
}

// Validator returns a schema that can validate objects stored on disk.
func (s *SingletonClients) Validator(
	validationDirective string,
) (validation.Schema, error) {
	if validationDirective == metav1.FieldValidationIgnore {
		return validation.NullSchema{}, nil
	}

	resources, err := s.OpenAPISchema()
	if err != nil {
		return nil, err
	}

	conjSchema := validation.ConjunctiveSchema{
		validation.NewSchemaValidation(resources),
		validation.NoDoubleKeySchema{},
	}
	return validation.NewParamVerifyingSchema(conjSchema, nil, validationDirective), nil
}
