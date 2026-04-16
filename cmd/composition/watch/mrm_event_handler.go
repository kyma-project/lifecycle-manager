package watch

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	"github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

func ComposeMrmEventHandler(clnt client.Client, maxUpdateDelay time.Duration) handler.EventHandler {
	kymaRepository := kyma.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	return modulereleasemeta.NewEventHandler(kymaRepository, maxUpdateDelay)
}
