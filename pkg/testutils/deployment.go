package testutils

import (
	"context"
	"errors"
	"fmt"

	apiappsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrDeploymentNotReady = errors.New("deployment is not ready")
	ErrDeploymentUpdating = errors.New("deployment is still updating")
)

func DeploymentIsReady(ctx context.Context, name, namespace string, clnt client.Client) error {
	deploy, err := GetDeployment(ctx, clnt, name, namespace)
	if err != nil {
		if util.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("could not get deployment: %w", err)
	}

	if deploy.Spec.Replicas != nil &&
		*deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		return nil
	}
	return ErrDeploymentNotReady
}

func StopDeployment(ctx context.Context, clnt client.Client,
	name, namespace string,
) error {
	deploy, err := GetDeployment(ctx, clnt, name, namespace)
	if err != nil {
		return fmt.Errorf("could not get deployment: %w", err)
	}
	if deploy.Status.AvailableReplicas == 0 {
		return nil
	}
	deploy.Spec.Replicas = int32Ptr(0)
	if err := clnt.Update(ctx, deploy); err != nil {
		return fmt.Errorf("could not update deployment: %w", err)
	}
	return ErrDeploymentUpdating
}

func EnableDeployment(ctx context.Context, clnt client.Client,
	name, namespace string,
) error {
	deploy, err := GetDeployment(ctx, clnt, name, namespace)
	if err != nil {
		return fmt.Errorf("could not get deployment: %w", err)
	}
	if deploy.Status.AvailableReplicas != 0 {
		return nil
	}
	deploy.Spec.Replicas = int32Ptr(1)
	if err := clnt.Update(ctx, deploy); err != nil {
		return fmt.Errorf("could not update deployment: %w", err)
	}
	return ErrDeploymentUpdating
}

func GetDeployment(ctx context.Context, clnt client.Client,
	name, namespace string,
) (*apiappsv1.Deployment, error) {
	deploy := &apiappsv1.Deployment{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deploy); err != nil {
		return nil, fmt.Errorf("could not get deployment: %w", err)
	}
	return deploy, nil
}
func int32Ptr(i int32) *int32 { return &i }
