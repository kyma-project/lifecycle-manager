package deletion

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	manifestrepo "github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
)

func ComposeDeletionService(clnt client.Client, eventHandler event.Event) *deletion.Service {
	mrmRepo := modulereleasemeta.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	manifestRepo := manifestrepo.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	ensureFinalizerUseCase := usecases.NewEnsureFinalizer(mrmRepo, eventHandler)
	skipNonDeletingUseCase := usecases.NewSkipNonDeleting()
	deleteManifestsUseCase := usecases.NewDeleteManifests(manifestRepo, eventHandler)
	removeFinalizerUseCase := usecases.NewRemoveFinalizer(mrmRepo)

	return deletion.NewService(ensureFinalizerUseCase, skipNonDeletingUseCase, deleteManifestsUseCase,
		removeFinalizerUseCase)
}
