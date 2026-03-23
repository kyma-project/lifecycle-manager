package oci

import (
	"os"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/repository/ocm"
	"github.com/kyma-project/lifecycle-manager/internal/repository/ocm/oci"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
)

func ComposeRepository(
	kcl spec.KeyChainLookup,
	ociRegistry *setup.OCIRegistry,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *ocm.RepositoryReader {
	ociRepository, err := oci.NewRepository(kcl, ociRegistry.IsInsecure())
	if err != nil {
		logger.Error(err, "failed to create OCI repository")
		os.Exit(bootstrapFailedExitCode)
	}
	ocmRepository, err := ocm.NewRepository(ociRegistry.GetReference(), ociRepository)
	if err != nil {
		logger.Error(err, "failed to create OCI repository")
		os.Exit(bootstrapFailedExitCode)
	}
	return ocmRepository
}
