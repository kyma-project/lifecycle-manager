package installation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	"github.com/kyma-project/lifecycle-manager/internal/repository/moduletemplate"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/installation"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
)

func ComposeInstallationService(clnt client.Client,
	descriptorProvider *provider.CachedDescriptorProvider,
	ociRegistryHost string,
	remoteSyncNamespace string,
	metrics *metrics.MandatoryModulesMetrics,
) *installation.Service {
	mrmRepo := modulereleasemeta.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	mtRepo := moduletemplate.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	moduleParser := parser.NewParser(clnt, descriptorProvider, remoteSyncNamespace, ociRegistryHost)
	manifestCreator := sync.New(clnt)
	return installation.NewService(mrmRepo, mtRepo, moduleParser, manifestCreator, metrics)
}
