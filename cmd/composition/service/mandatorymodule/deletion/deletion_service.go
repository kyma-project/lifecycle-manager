package deletion

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	manifestrepository "github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	modulereleasemetarepository "github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ComposeDeletionService(clnt client.Client, eventHandler event.Event) *deletion.Service {
	mrmRepo := modulereleasemetarepository.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	manifestRepo := manifestrepository.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	ensureFinalizerUseCase := usecases.NewEnsureFinalizer(mrmRepo, eventHandler)
	deleteManifestsUseCase := usecases.NewDeleteManifests(manifestRepo, eventHandler)
	removeFinalizerUseCase := usecases.NewRemoveFinalizer(mrmRepo)

	return deletion.NewService(ensureFinalizerUseCase, deleteManifestsUseCase,
		removeFinalizerUseCase)
}
