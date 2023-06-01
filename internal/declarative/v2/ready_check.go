package v2

import (
	"context"
	"errors"

	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrResourcesNotReady           = errors.New("resources are not ready")
	ErrCustomResourceStateNotFound = errors.New("custom resource state not found")
	ErrDeploymentNotReady          = errors.New("deployment is not ready")
)

type ReadyCheck interface {
	Run(ctx context.Context, clnt Client, obj Object, resources []*resource.Info) error
}

func NewExistsReadyCheck() ReadyCheck {
	return &ExistsReadyCheck{}
}

type ExistsReadyCheck struct{}

func (c *ExistsReadyCheck) Run(ctx context.Context, clnt Client, _ Object, resources []*resource.Info) error {
	for i := range resources {
		obj, ok := resources[i].Object.(client.Object)
		if !ok {
			//nolint:goerr113
			return errors.New("object in resource info is not a valid client object")
		}
		if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}
