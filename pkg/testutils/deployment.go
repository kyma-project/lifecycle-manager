package testutils

import (
	"context"
	"errors"
	"fmt"

	apiappsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrDeploymentNotReady = errors.New("deployment is not ready")

func DeploymentIsReady(ctx context.Context, name, namespace string, clnt client.Client) error {
	deploy := &apiappsv1.Deployment{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, deploy); err != nil {
		return fmt.Errorf("could not get deployment: %w", err)
	}

	if deploy.Spec.Replicas != nil &&
		*deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		return nil
	}
	return ErrDeploymentNotReady
}
