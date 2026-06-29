package statecheck

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

// ExistsStateCheck reports StateReady once every resource passed in is
// retrievable from the SKR cluster. Used by integration suites that don't
// have a Deployment/StatefulSet manager to track.
type ExistsStateCheck struct{}

func NewExistsStateCheck() *ExistsStateCheck {
	return &ExistsStateCheck{}
}

func (c *ExistsStateCheck) GetState(
	ctx context.Context,
	clnt client.Client,
	resources []client.Object,
) (shared.State, error) {
	for _, obj := range resources {
		if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
			return shared.StateError, fmt.Errorf("failed to fetch object by key: %w", err)
		}
	}
	return shared.StateReady, nil
}
