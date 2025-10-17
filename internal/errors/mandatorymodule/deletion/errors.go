package deletion

import "errors"

var ErrMrmNotInDeletingState = errors.New("ModuleReleaseMeta not in deleting state")
