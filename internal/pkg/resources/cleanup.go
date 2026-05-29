package resources

import (
	"context"
	"errors"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrDeletionNotFinished = errors.New("deletion is not yet finished")

type ConcurrentCleanup struct {
	clnt     client.Client
	manifest *v1beta2.Manifest
}

func NewConcurrentCleanup(clnt client.Client, manifest *v1beta2.Manifest) *ConcurrentCleanup {
	return &ConcurrentCleanup{
		clnt:     clnt,
		manifest: manifest,
	}
}

func (c *ConcurrentCleanup) DeleteDiffResources(ctx context.Context, resources []shared.Resource) error {
	status := c.manifest.GetStatus()
	operatorRelatedResources, operatorManagedResources := SplitResources(resources)
	if err := c.cleanupResources(ctx, operatorManagedResources, status); err != nil {
		return err
	}
	return c.cleanupResources(ctx, operatorRelatedResources, status)
}

func SplitResources(resources []shared.Resource) ([]shared.Resource, []shared.Resource) {
	operatorRelatedResources := make([]shared.Resource, 0)
	operatorManagedResources := make([]shared.Resource, 0)

	for _, res := range resources {
		if IsOperatorRelatedResources(res.Kind) {
			operatorRelatedResources = append(operatorRelatedResources, res)
			continue
		}
		operatorManagedResources = append(operatorManagedResources, res)
	}

	return operatorRelatedResources, operatorManagedResources
}

func IsOperatorRelatedResources(kind string) bool {
	return kind == "CustomResourceDefinition" ||
		kind == "Namespace" ||
		kind == "ServiceAccount" ||
		kind == "Role" ||
		kind == "ClusterRole" ||
		kind == "RoleBinding" ||
		kind == "ClusterRoleBinding" ||
		kind == "Service" ||
		kind == "Deployment"
}

func (c *ConcurrentCleanup) Run(ctx context.Context, resources []shared.Resource) error {
	count := len(resources)
	results := make(chan error, count)
	for i := range resources {
		go c.cleanupResource(ctx, resources[i], results)
	}

	var errs []error
	for range resources {
		err := <-results
		if util.IsNotFound(err) {
			count--
			continue
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if count > 0 {
		return ErrDeletionNotFinished
	}
	return nil
}

func (c *ConcurrentCleanup) cleanupResources(
	ctx context.Context,
	resources []shared.Resource,
	status shared.Status,
) error {
	if err := c.Run(ctx, resources); errors.Is(err, ErrDeletionNotFinished) {
		c.manifest.SetStatus(status.WithState(shared.StateWarning).WithErr(err))
		return err
	} else if err != nil {
		c.manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return err
	}
	return nil
}

func (c *ConcurrentCleanup) cleanupResource(ctx context.Context, res shared.Resource, results chan error) {
	results <- c.clnt.Delete(ctx, res.ToUnstructured(),
		client.PropagationPolicy(apimetav1.DeletePropagationBackground))
}
