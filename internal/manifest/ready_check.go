package manifest

import (
	"context"
	"fmt"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

func NewResourceReadyCheck() *ResourceReadyCheck {
	return &ResourceReadyCheck{}
}

type ResourceReadyCheck struct{}

type ResourceKind string

const (
	DeploymentKind  ResourceKind = "Deployment"
	StatefulSetKind ResourceKind = "StatefulSet"
)

type Resource struct {
	Kind ResourceKind
	*apiappsv1.Deployment
	*apiappsv1.StatefulSet
}

func (c *ResourceReadyCheck) Run(ctx context.Context,
	clnt declarativev2.Client,
	resources []*resource.Info,
) (shared.State, error) {
	res := findResource(clnt, resources)
	if res == nil {
		return shared.StateReady, nil
	}

	switch res.Kind {
	case StatefulSetKind:
		statefulSetReadyCheck := NewStatefulSetReadyCheck()
		return statefulSetReadyCheck.Run(ctx, clnt, res.StatefulSet)
	case DeploymentKind:
		deploymentReadyCheck := NewDeploymentReadyCheck()
		return deploymentReadyCheck.Run(ctx, clnt, res.Deployment)
	}

	return shared.StateReady, nil
}

func findResource(clt declarativev2.Client, resources []*resource.Info) *Resource {
	deploy := &apiappsv1.Deployment{}
	statefulSet := &apiappsv1.StatefulSet{}

	for _, res := range resources {
		if err := clt.Scheme().Convert(res.Object, deploy, nil); err == nil {
			return &Resource{
				Kind:       DeploymentKind,
				Deployment: deploy,
			}
		}

		if err := clt.Scheme().Convert(res.Object, statefulSet, nil); err == nil {
			return &Resource{
				Kind:        StatefulSetKind,
				StatefulSet: statefulSet,
			}
		}
	}

	return nil
}

func getPodsList(ctx context.Context, clt declarativev2.Client, namespace string,
	matchLabels map[string]string) (*apicorev1.PodList,
	error,
) {
	podList := &apicorev1.PodList{}
	listOptions := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: k8slabels.SelectorFromSet(matchLabels),
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
