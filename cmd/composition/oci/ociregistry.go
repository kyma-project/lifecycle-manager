package oci

import (
	"context"
	"os"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
)

// ComposeOCIRegistry creates a new OCIRegistry instance from the provided secret repository and flag configuration.
func ComposeOCIRegistry(
	secretRepo *secretrepo.Repository,
	flagVar *flags.FlagVar,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *setup.OCIRegistry {
	ociRegistry, err := setup.NewOCIRegistry(context.Background(),
		secretRepo,
		flagVar.OciRegistryHost,
		flagVar.OciRegistryCredSecretName,
		flagVar.ModulesRepositorySubPath,
	)
	if err != nil {
		logger.Error(err, "failed to setup OCI registry")
		os.Exit(bootstrapFailedExitCode)
	}
	return ociRegistry
}
