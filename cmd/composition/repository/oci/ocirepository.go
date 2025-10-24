package oci

import (
	"os"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/repository/oci"
)

func ComposeOCIRepository(
	kcl spec.KeyChainLookup,
	hostWithPort string,
	insecure bool,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *oci.RepositoryReader {
	repository, err := oci.NewRepository(kcl, hostWithPort, insecure, &oci.DefaultCraneWrapper{})
	if err != nil {
		logger.Error(err, "failed to create OCI repository")
		os.Exit(bootstrapFailedExitCode)
	}
	return repository
}
