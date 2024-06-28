package manifest

import (
	"context"
	"fmt"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/util/deployment"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	resources []*resource.Info,
) (declarativev2.StateInfo, error) {
	deploymentState := getDeploymentState(ctx, clnt, resources)
	return declarativev2.StateInfo{State: deploymentState}, nil
}

func getDeploymentState(ctx context.Context, clt declarativev2.Client, resources []*resource.Info) shared.State {
	deploy, found := findDeployment(clt, resources)
	// Not every module operator use Deployment by default, e.g: StatefulSet also a valid approach
	if !found {
		return shared.StateReady
	}

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

func findDeployment(clt declarativev2.Client, resources []*resource.Info) (*apiappsv1.Deployment, bool) {
	deploy := &apiappsv1.Deployment{}
	for _, res := range resources {
		if err := clt.Scheme().Convert(res.Object, deploy, nil); err == nil {
			return deploy, true
		}
	}
	return nil, false
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
	podList := &apicorev1.PodList{}
	listOptions := &client.ListOptions{
		Namespace:     deploy.Namespace,
		LabelSelector: k8slabels.SelectorFromSet(deploy.Spec.Selector.MatchLabels),
	}
	if err := clt.List(ctx, podList, listOptions); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return podList, nil
}

func GetPodsState(podList *apicorev1.PodList) shared.State {
	for _, pod := range podList.Items {
		for _, condition := range pod.Status.ContainerStatuses {
			if condition.Started == nil {
				return shared.StateError
			}
			switch {
			case *condition.Started && condition.Ready:
				return shared.StateReady
			case *condition.Started && !condition.Ready:
				return shared.StateProcessing
			default:
				return shared.StateError
			}
		}
	}
	return shared.StateError
}
