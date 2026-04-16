package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	mtrepo "github.com/kyma-project/lifecycle-manager/internal/repository/moduletemplate"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
)

func ComposeTemplateChangeHandlerMapFunc(clnt client.Client) handler.MapFunc {
	templateRepo := mtrepo.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	kymaRepo := kymarepo.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	return watch.NewTemplateChangeHandler(templateRepo, kymaRepo).Watch()
}
