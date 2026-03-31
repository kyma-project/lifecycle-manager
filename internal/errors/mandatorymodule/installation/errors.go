package installation

import "errors"

var (
	ErrSkippingReconciliationKyma = errors.New("skipping reconciliation for Kyma")
	ErrKymaBeingDeleted           = errors.New("skipping installation for Kyma being deleted")
)
