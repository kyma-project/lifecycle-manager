package skrclient

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func (s *Service) ToRESTConfig() (*rest.Config, error) {
	return s.config, nil
}

func (s *Service) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return s.discoveryClient, nil
}

func (s *Service) ToRESTMapper() (meta.RESTMapper, error) {
	return s.discoveryShortcutExpander, nil
}

func (s *Service) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}
