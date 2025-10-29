package installation

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	modulereleasemetarepository "github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	moduletemplaterepository "github.com/kyma-project/lifecycle-manager/internal/repository/moduletemplate"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/installation"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ComposeInstallationService(clnt client.Client,
	descriptorProvider *provider.CachedDescriptorProvider,
	ociRegistryHost string,
	remoteSyncNamespace string,
	metrics *metrics.MandatoryModulesMetrics,
) *installation.Service {
	mrmRepo := modulereleasemetarepository.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	mtRepo := moduletemplaterepository.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	moduleParser := parser.NewParser(clnt, descriptorProvider, remoteSyncNamespace, ociRegistryHost)
	manifestCreator := sync.New(clnt)
	return installation.NewService(mrmRepo, mtRepo, moduleParser, manifestCreator, metrics)
}
