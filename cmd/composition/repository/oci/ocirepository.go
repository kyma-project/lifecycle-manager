package oci

import (
	"os"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/repository/ocm"
	"github.com/kyma-project/lifecycle-manager/internal/repository/ocm/oci"
)

func ComposeOCIRepository(
	kcl spec.KeyChainLookup,
	hostWithPort string,
	insecure bool,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *ocm.RepositoryReader {
	ociRepository, err := oci.NewRepository(kcl, insecure)
	if err != nil {
		logger.Error(err, "failed to create OCI repository")
		os.Exit(bootstrapFailedExitCode)
	}
	ocmRepository, err := ocm.NewRepository(hostWithPort, ociRepository)
	if err != nil {
		logger.Error(err, "failed to create OCI repository")
		os.Exit(bootstrapFailedExitCode)
	}
	return ocmRepository
}
