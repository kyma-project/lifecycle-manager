package v2

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNotValidClientObject = errors.New("object in resource info is not a valid client object")

type StateInfo struct {
	State
	Info string
}

type ReadyCheck interface {
	Run(ctx context.Context, clnt Client, obj Object, resources []*resource.Info) (StateInfo, error)
}

func NewExistsReadyCheck() ReadyCheck {
	return &ExistsReadyCheck{}
}

type ExistsReadyCheck struct{}

func (c *ExistsReadyCheck) Run(
	ctx context.Context,
	clnt Client,
	_ Object,
	resources []*resource.Info,
) (StateInfo, error) {
	for i := range resources {
		obj, ok := resources[i].Object.(client.Object)
		if !ok {
			return StateInfo{State: StateError}, ErrNotValidClientObject
		}
		if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
			return StateInfo{State: StateError}, fmt.Errorf("failed to fetch object by key: %w", err)
		}
	}
	return StateInfo{State: StateReady}, nil
}
