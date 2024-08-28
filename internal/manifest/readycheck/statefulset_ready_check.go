package readycheck

import (
	"context"
	"fmt"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

// NewStatefulSetReadyCheck creates a readiness check that verifies if a StatefulSet is ready.
func NewStatefulSetReadyCheck() *StatefulSetReadyCheck {
	return &StatefulSetReadyCheck{}
}

type StatefulSetReadyCheck struct{}

func (c *StatefulSetReadyCheck) Run(ctx context.Context,
	clnt declarativev2.Client,
	statefulSet *apiappsv1.StatefulSet,
) (shared.State, error) {
	statefulSetState := getStatefulSetState(ctx, clnt, statefulSet)
	return statefulSetState, nil
}

func getStatefulSetState(ctx context.Context, clt declarativev2.Client,
	statefulSet *apiappsv1.StatefulSet,
) shared.State {
	if IsStatefulSetReady(statefulSet) {
		return shared.StateReady
	}

	podList, err := getPodsForStatefulSet(ctx, clt, statefulSet)
	if err != nil {
		return shared.StateError
	}

	return GetPodsState(podList)
}

func IsStatefulSetReady(statefulSet *apiappsv1.StatefulSet) bool {
	if statefulSet.Spec.Replicas != nil && *statefulSet.Spec.Replicas == statefulSet.Status.ReadyReplicas {
		return true
	}
	return false
}

func getPodsForStatefulSet(ctx context.Context, clt declarativev2.Client,
	statefulSet *apiappsv1.StatefulSet,
) (*apicorev1.PodList, error) {
	return getPodsList(ctx, clt, statefulSet.Namespace, statefulSet.Spec.Selector.MatchLabels)
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
		for _, status := range pod.Status.ContainerStatuses {
			if status.Started == nil {
				return shared.StateError
			}

			if status.State.Waiting != nil {
				return shared.StateProcessing
			}

			if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
				return shared.StateError
			}

			if !status.Ready {
				return shared.StateProcessing
			}
		}
	}
	return shared.StateReady
}
