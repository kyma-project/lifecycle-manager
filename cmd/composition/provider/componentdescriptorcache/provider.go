package componentdescriptorcache

import (
	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/cmd/composition/repository/oci"
	"github.com/kyma-project/lifecycle-manager/cmd/composition/service/componentdescriptor"
	descriptorcache "github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

// ComposeCachedDescriptorProvider manages creation of a new instance of the cached ComponentDescriptor provider
// including all of its dependencies (OCI repository, component descriptor service, descriptor cache).
func ComposeCachedDescriptorProvider(
	kcl spec.KeyChainLookup,
	ociRegistryHost string,
	insecure bool,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *provider.CachedDescriptorProvider {
	ocmDescriptorRepository := oci.ComposeOCIRepository(
		kcl,
		ociRegistryHost,
		insecure,
		logger,
		bootstrapFailedExitCode,
	)
	ocmDescriptorService := componentdescriptor.ComposeComponentDescriptorService(
		ocmDescriptorRepository,
		logger,
		bootstrapFailedExitCode,
	)
	return provider.NewCachedDescriptorProvider(
		ocmDescriptorService,
		descriptorcache.NewDescriptorCache(),
	)
}
