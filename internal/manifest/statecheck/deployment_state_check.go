package statecheck

import (
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/deployment"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	NewReplicaSetAvailableReason = "NewReplicaSetAvailable"
	FoundNewReplicaSetReason     = "FoundNewReplicaSet"
	NewReplicaSetCreatedReason   = "NewReplicaSetCreated"
	ReplicaSetUpdatedReason      = "ReplicaSetUpdated"
)

type DeploymentStateCheck struct{}

func NewDeploymentStateCheck() *DeploymentStateCheck {
	return &DeploymentStateCheck{}
}

func (*DeploymentStateCheck) GetState(
	deploy *apiappsv1.Deployment,
) (shared.State, error) {
	progressingCondition := deployment.GetDeploymentCondition(deploy.Status, apiappsv1.DeploymentProgressing)
	availableCondition := deployment.GetDeploymentCondition(deploy.Status, apiappsv1.DeploymentAvailable)

	deploymentState := determineDeploymentState(progressingCondition, availableCondition)
	return deploymentState, nil
}

func determineDeploymentState(progressingCondition, availableCondition *apiappsv1.DeploymentCondition) shared.State {
	isAvailable := availableCondition != nil && availableCondition.Status == apicorev1.ConditionTrue
	isProgressing := progressingCondition != nil && progressingCondition.Status == apicorev1.ConditionTrue
	if isProgressing {
		if isAvailable {
			switch progressingCondition.Reason {
			case NewReplicaSetAvailableReason:
				return shared.StateReady
			case FoundNewReplicaSetReason, NewReplicaSetCreatedReason, ReplicaSetUpdatedReason:
				return shared.StateProcessing
			}
		} else {
			switch progressingCondition.Reason {
			case NewReplicaSetAvailableReason, FoundNewReplicaSetReason,
				NewReplicaSetCreatedReason, ReplicaSetUpdatedReason:
				return shared.StateProcessing
			}
		}
	}

	return shared.StateError
}
