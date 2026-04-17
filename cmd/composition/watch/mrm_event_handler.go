package watch

import (
	"time"

	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	mrmwatch "github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta"
)

func ComposeMrmEventHandler(kymaRepo *kymarepo.Repository, maxUpdateDelay time.Duration) *mrmwatch.EventHandler {
	return mrmwatch.NewEventHandler(kymaRepo, maxUpdateDelay)
}
