package shared

const (
	BlockingKymaDeletionFinalizer = OperatorGroup + Separator + "background-deletion"
	KymaFinalizer                 = OperatorGroup + Separator + string(KymaKind)
	PurgeFinalizer                = OperatorGroup + Separator + "purge-finalizer"
	WatcherFinalizer              = OperatorGroup + Separator + "watcher"
)
