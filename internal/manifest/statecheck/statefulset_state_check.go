package statecheck

import (
	"context"
	"fmt"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type StatefulSetStateCheck struct{}

func NewStatefulSetStateCheck() *StatefulSetStateCheck {
	return &StatefulSetStateCheck{}
}

func (*StatefulSetStateCheck) GetState(ctx context.Context,
	clnt client.Client,
	statefulSet *apiappsv1.StatefulSet,
) (shared.State, error) {
	if IsStatefulSetReady(statefulSet) {
		return shared.StateReady, nil
	}

	podList, err := getPodsForStatefulSet(ctx, clnt, statefulSet)
	if err != nil {
		return shared.StateError, err
	}

	statefulSetState := GetPodsState(podList)
	return statefulSetState, nil
}

func IsStatefulSetReady(statefulSet *apiappsv1.StatefulSet) bool {
	if statefulSet.Spec.Replicas != nil && *statefulSet.Spec.Replicas == statefulSet.Status.ReadyReplicas {
		return true
	}
	return false
}

func getPodsForStatefulSet(ctx context.Context, clt client.Client,
	statefulSet *apiappsv1.StatefulSet,
) (*apicorev1.PodList, error) {
	return getPodsList(ctx, clt, statefulSet.Namespace, statefulSet.Spec.Selector.MatchLabels)
}

func getPodsList(ctx context.Context, clt client.Client, namespace string,
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
