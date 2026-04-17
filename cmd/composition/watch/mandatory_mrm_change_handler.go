package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/handler"

	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	mrmrepo "github.com/kyma-project/lifecycle-manager/internal/repository/modulereleasemeta"
	mrmwatch "github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta"
)

func ComposeMandatoryMrmChangeHandlerMapFunc(mrmRepo *mrmrepo.Repository, kymaRepo *kymarepo.Repository) handler.MapFunc {
	return mrmwatch.NewMandatoryMrmChangeHandler(mrmRepo, kymaRepo).Watch()
}
