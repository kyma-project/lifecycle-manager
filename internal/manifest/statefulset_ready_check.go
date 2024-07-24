package manifest

import (
	"context"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"

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

	// Since statefulset is not ready, check if pods are ready or in error state
	// Get all Pods associated with the StatefulSet
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
