package componentdescriptorcache

import (
	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/cmd/composition/repository/oci"
	"github.com/kyma-project/lifecycle-manager/cmd/composition/service/componentdescriptor"
	descriptorcache "github.com/kyma-project/lifecycle-manager/internal/descriptor/cache"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
)

// ComposeCachedDescriptorProvider manages creation of a new instance of the cached ComponentDescriptor provider
// including all of its dependencies (OCI repository, component descriptor service, descriptor cache).
func ComposeCachedDescriptorProvider(
	kcl spec.KeyChainLookup,
	ociRegistry *setup.OCIRegistry,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *provider.CachedDescriptorProvider {
	ocmDescriptorRepository := oci.ComposeOCIRepository(
		kcl,
		ociRegistry,
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
