package deletion

import "errors"

var (
	ErrMrmNotMandatory       = errors.New("ModuleReleaseMeta is not a mandatory module")
	ErrMrmNotInDeletingState = errors.New("ModuleReleaseMeta not in deleting state")
)
