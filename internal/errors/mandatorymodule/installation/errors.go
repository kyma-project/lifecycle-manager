package installation

import "errors"

var (
	ErrSkipReconcileKyma = errors.New("skip mandatory module installation for Kyma with skip-reconciliation label")
	ErrKymaBeingDeleted  = errors.New("skip mandatory module installation for Kyma being deleted")
)
