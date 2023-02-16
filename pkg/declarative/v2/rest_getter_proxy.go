package v2

import (
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Checking compliance with the interface methods implemented below.
var _ action.RESTClientGetter = &SingletonClients{}

func (s *SingletonClients) ToRESTConfig() (*rest.Config, error) {
	return s.config, nil
}

func (s *SingletonClients) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return s.discoveryClient, nil
}

func (s *SingletonClients) ToRESTMapper() (meta.RESTMapper, error) {
	return s.discoveryShortcutExpander, nil
}

func (s *SingletonClients) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}
