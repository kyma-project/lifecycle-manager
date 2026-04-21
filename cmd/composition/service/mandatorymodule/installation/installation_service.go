package installation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	mrmrepo "github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	mtrepo "github.com/kyma-project/lifecycle-manager/internal/repository/moduletemplate"
	installservice "github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/installation"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
)

func ComposeInstallationService(clnt client.Client,
	mrmRepo *mrmrepo.Repository,
	mtRepo *mtrepo.Repository,
	descriptorProvider *provider.CachedDescriptorProvider,
	ociRegistry string,
	remoteSyncNamespace string,
	metrics *metrics.MandatoryModulesMetrics,
) *installservice.Service {
	moduleParser := parser.NewParser(clnt, descriptorProvider, remoteSyncNamespace, ociRegistry)
	manifestCreator := sync.New(clnt)
	return installservice.NewService(mrmRepo, mtRepo, moduleParser, manifestCreator, metrics)
}
