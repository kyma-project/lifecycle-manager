package componentdescriptorcache

import (
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
)

// ComposeCachedDescriptorProvider manages creation of a new instance of the cached ComponentDescriptor provider.
func ComposeCachedDescriptorProvider(
	service provider.DescriptorService,
	cache provider.DescriptorCache,
) *provider.CachedDescriptorProvider {
	return provider.NewCachedDescriptorProvider(service, cache)
}
