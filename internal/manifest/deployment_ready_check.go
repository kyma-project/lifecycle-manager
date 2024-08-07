package manifest

import (
	"context"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/deployment"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

// NewDeploymentReadyCheck creates a readiness check that verifies if a Deployment is ready.
func NewDeploymentReadyCheck() *DeploymentReadyCheck {
	return &DeploymentReadyCheck{}
}

type DeploymentReadyCheck struct{}

func (c *DeploymentReadyCheck) Run(ctx context.Context,
	clnt declarativev2.Client,
	deploy *apiappsv1.Deployment,
) (shared.State, error) {
	deploymentState := getDeploymentState(ctx, clnt, deploy)
	return deploymentState, nil
}

func getDeploymentState(ctx context.Context, clt declarativev2.Client, deploy *apiappsv1.Deployment) shared.State {
	if IsDeploymentReady(deploy) {
		return shared.StateReady
	}

	// Since deployment is not ready check if pods are ready or in error state
	// Get all Pods associated with the Deployment
	podList, err := getPodsForDeployment(ctx, clt, deploy)
	if err != nil {
		return shared.StateError
	}

	return GetPodsState(podList)
}

func IsDeploymentReady(deploy *apiappsv1.Deployment) bool {
	availableCond := deployment.GetDeploymentCondition(deploy.Status, apiappsv1.DeploymentAvailable)
	if availableCond != nil && availableCond.Status == apicorev1.ConditionTrue {
		return true
	}
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		return true
	}
	return false
}

func getPodsForDeployment(ctx context.Context, clt declarativev2.Client,
	deploy *apiappsv1.Deployment,
) (*apicorev1.PodList, error) {
	return getPodsList(ctx, clt, deploy.Namespace, deploy.Spec.Selector.MatchLabels)
}
