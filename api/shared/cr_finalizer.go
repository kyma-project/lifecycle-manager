package shared

const (
	KymaFinalizer            = OperatorGroup + Separator + string(KymaKind)
	WatcherFinalizer         = OperatorGroup + Separator + "watcher"
	MandatoryModuleFinalizer = OperatorGroup + Separator + "mandatory-module"
)
