package v2

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

var ErrNotValidClientObject = errors.New("object in resource info is not a valid client object")

type ReadyCheck interface {
	Run(ctx context.Context, clnt Client, resources []*resource.Info) (shared.State, error)
}

func NewExistsReadyCheck() *ExistsReadyCheck {
	return &ExistsReadyCheck{}
}

type ExistsReadyCheck struct{}

func (c *ExistsReadyCheck) Run(
	ctx context.Context,
	clnt Client,
	resources []*resource.Info,
) (shared.State, error) {
	for i := range resources {
		obj, ok := resources[i].Object.(client.Object)
		if !ok {
			return shared.StateError, ErrNotValidClientObject
		}
		if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
			return shared.StateError, fmt.Errorf("failed to fetch object by key: %w", err)
		}
	}
	return shared.StateReady, nil
}
