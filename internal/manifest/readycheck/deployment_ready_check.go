package readycheck

import (
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/deployment"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	NewRSAvailableReason    = "NewReplicaSetAvailable"
	FoundNewRSReason        = "FoundNewReplicaSet"
	NewReplicaSetReason     = "NewReplicaSetCreated"
	ReplicaSetUpdatedReason = "ReplicaSetUpdated"
)

// NewDeploymentReadyCheck creates a readiness check that verifies if a Deployment is ready.
func NewDeploymentReadyCheck() *DeploymentReadyCheck {
	return &DeploymentReadyCheck{}
}

type DeploymentReadyCheck struct{}

func (c *DeploymentReadyCheck) Run(
	deploy *apiappsv1.Deployment,
) (shared.State, error) {
	deploymentState := GetDeploymentState(deploy)
	return deploymentState, nil
}

func GetDeploymentState(deploy *apiappsv1.Deployment) shared.State {
	progressingCondition := deployment.GetDeploymentCondition(deploy.Status, apiappsv1.DeploymentProgressing)
	availableCondition := deployment.GetDeploymentCondition(deploy.Status, apiappsv1.DeploymentAvailable)

	return determineDeploymentState(progressingCondition, availableCondition)
}

func determineDeploymentState(progressingCondition, availableCondition *apiappsv1.DeploymentCondition) shared.State {
	isAvailable := availableCondition != nil && availableCondition.Status == apicorev1.ConditionTrue
	isProgressing := progressingCondition != nil && progressingCondition.Status == apicorev1.ConditionTrue
	if isProgressing {
		if isAvailable {
			switch progressingCondition.Reason {
			case NewRSAvailableReason:
				return shared.StateReady
			case FoundNewRSReason, NewReplicaSetReason, ReplicaSetUpdatedReason:
				return shared.StateProcessing
			}
		} else {
			switch progressingCondition.Reason {
			case NewRSAvailableReason, FoundNewRSReason, NewReplicaSetReason, ReplicaSetUpdatedReason:
				return shared.StateProcessing
			}
		}
	}

	return shared.StateError
}
