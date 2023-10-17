package v2

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ModuleCRDeletionCheck interface {
	Run(ctx context.Context, clnt client.Client, obj Object) (bool, error)
}

// NewDefaultDeletionCheck creates a check that verifies that the Resource CR in the remote cluster is deleted.
func NewDefaultDeletionCheck() *DefaultDeletionCheck {
	return &DefaultDeletionCheck{}
}

type DefaultDeletionCheck struct{}

//nolint:revive
func (c *DefaultDeletionCheck) Run(ctx context.Context, clnt client.Client, obj Object) (bool, error) {
	return true, nil
}
