package shared

const (
	KymaFinalizer    = OperatorGroup + Separator + string(KymaKind)
	PurgeFinalizer   = OperatorGroup + Separator + "purge-finalizer"
	WatcherFinalizer = OperatorGroup + Separator + "watcher"
)
