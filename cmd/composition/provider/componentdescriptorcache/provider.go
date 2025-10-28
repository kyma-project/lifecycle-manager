package componentdescriptorcache

import (
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
)

// ComposeComponentDescriptorService manges creation of a new instance of the Cached ComponentDescriptor Provider.
func ComposeCachedDescriptorProvider(
	service provider.DescriptorService,
	cache provider.DescriptorCache,
) *provider.CachedDescriptorProvider {
	return provider.NewCachedDescriptorProvider(service, cache)
}
