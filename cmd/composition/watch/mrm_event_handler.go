package watch

import (
	"time"

	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	mrmwatch "github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta"
	"github.com/kyma-project/lifecycle-manager/internal/watch/modulereleasemeta/events"
)

func ComposeMrmEventHandler(kymaRepo *kymarepo.Repository, maxUpdateDelay time.Duration) *mrmwatch.EventHandler {
	return mrmwatch.NewEventHandler(kymaRepo, events.RegularResolver{}, maxUpdateDelay)
}

func ComposeMandatoryMrmEventHandler(kymaRepo *kymarepo.Repository,
	maxUpdateDelay time.Duration,
) *mrmwatch.EventHandler {
	return mrmwatch.NewEventHandler(kymaRepo, events.MandatoryResolver{}, maxUpdateDelay)
}
