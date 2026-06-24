package oci

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
)

// ComposeRegistry faciliates the creation of a new OCIRegistry.
// The kcpClientWithoutCache is an uncached client because the registry resolves a secret
// before the manager is started.
func ComposeRegistry(
	kcpClientWithoutCache client.Client,
	flagVar *flags.FlagVar,
	logger logr.Logger,
	bootstrapFailedExitCode int,
) *setup.OCIRegistry {
	secretRepo := secretrepo.NewRepository(kcpClientWithoutCache, shared.DefaultControlPlaneNamespace)
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
