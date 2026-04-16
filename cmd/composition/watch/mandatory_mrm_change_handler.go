package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	mrmrepo "github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	mrmwatch "github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta"
)

func ComposeMandatoryMrmChangeHandlerMapFunc(clnt client.Client) handler.MapFunc {
	kymaRepo := kymarepo.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	mrm := mrmrepo.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	return mrmwatch.NewMandatoryMrmChangeHandler(mrm, kymaRepo).Watch()
}
