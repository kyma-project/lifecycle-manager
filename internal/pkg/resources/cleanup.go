package resources

import (
	"context"
	"errors"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
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

func (c *ConcurrentCleanup) DeleteDiffResources(ctx context.Context, resources []*resource.Info,
) error {
	status := c.manifest.GetStatus()
	operatorRelatedResources, operatorManagedResources, err := SplitResources(resources)
	if err != nil {
		return err
	}
	err = c.cleanupResources(ctx, operatorManagedResources, status)
	if err != nil {
		return err
	}
	return c.cleanupResources(ctx, operatorRelatedResources, status)
}

func SplitResources(resources []*resource.Info) ([]*resource.Info, []*resource.Info, error) {
	operatorRelatedResources := make([]*resource.Info, 0)
	operatorManagedResources := make([]*resource.Info, 0)

	for _, item := range resources {
		obj, ok := item.Object.(client.Object)
		if !ok {
			return nil, nil, common.ErrTypeAssert
		}
		if IsOperatorRelatedResources(obj.GetObjectKind().GroupVersionKind().Kind) {
			operatorRelatedResources = append(operatorRelatedResources, item)
			continue
		}
		operatorManagedResources = append(operatorManagedResources, item)
	}

	return operatorRelatedResources, operatorManagedResources, nil
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

func (c *ConcurrentCleanup) Run(ctx context.Context, infos []*resource.Info) error {
	infosCount := len(infos)
	results := make(chan error, infosCount)
	for i := range infos {
		go c.cleanupResource(ctx, infos[i], results)
	}

	var errs []error
	for range infos {
		err := <-results
		if util.IsNotFound(err) {
			infosCount--
			continue
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	if infosCount > 0 {
		return ErrDeletionNotFinished
	}
	return nil
}

func (c *ConcurrentCleanup) cleanupResources(
	ctx context.Context,
	resources []*resource.Info,
	status shared.Status,
) error {
	err := c.Run(ctx, resources)
	if errors.Is(err, ErrDeletionNotFinished) {
		c.manifest.SetStatus(status.WithState(shared.StateWarning).WithErr(err))
		return err
	} else if err != nil {
		c.manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return err
	}
	return nil
}

func (c *ConcurrentCleanup) cleanupResource(ctx context.Context, info *resource.Info, results chan error) {
	obj, ok := info.Object.(client.Object)
	if !ok {
		return
	}
	results <- c.clnt.Delete(ctx, obj, client.PropagationPolicy(apimetav1.DeletePropagationBackground))
}
