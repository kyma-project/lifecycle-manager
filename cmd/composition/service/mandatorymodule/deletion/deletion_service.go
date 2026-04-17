package deletion

import (
	"github.com/kyma-project/lifecycle-manager/internal/event"
	manifestrepo "github.com/kyma-project/lifecycle-manager/internal/repository/manifest"
	mrmrepo "github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule/deletion/usecases"
)

func ComposeDeletionService(mrmRepo *mrmrepo.Repository,
	manifestRepo *manifestrepo.Repository,
	eventHandler event.Event,
) *deletion.Service {
	ensureFinalizerUseCase := usecases.NewEnsureFinalizer(mrmRepo, eventHandler)
	skipNonDeletingUseCase := usecases.NewSkipNonDeleting()
	deleteManifestsUseCase := usecases.NewDeleteManifests(manifestRepo, eventHandler)
	removeFinalizerUseCase := usecases.NewRemoveFinalizer(mrmRepo)

	return deletion.NewService(ensureFinalizerUseCase, skipNonDeletingUseCase, deleteManifestsUseCase,
		removeFinalizerUseCase)
}
