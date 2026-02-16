package statecheck_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
)

func Test_IsStatefulSetReady(t *testing.T) {
	one := new(int32)
	*one = 1

	tests := []struct {
		name        string
		statefulSet *apiappsv1.StatefulSet
		expected    bool
	}{
		{
			name: "Test StatefulSet Ready",
			statefulSet: &apiappsv1.StatefulSet{
				Status: apiappsv1.StatefulSetStatus{
					ReadyReplicas: 1,
				},
				Spec: apiappsv1.StatefulSetSpec{
					Replicas: one,
				},
			},
			expected: true,
		},
		{
			name: "Test StatefulSet Not Ready",
			statefulSet: &apiappsv1.StatefulSet{
				Status: apiappsv1.StatefulSetStatus{
					ReadyReplicas: 0,
				},
				Spec: apiappsv1.StatefulSetSpec{
					Replicas: one,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, statecheck.IsStatefulSetReady(tt.statefulSet))
		})
	}
}

func Test_GetPodsState(t *testing.T) {
	startedTrue := new(bool)
	*startedTrue = true
	startedFalse := new(bool)
	*startedFalse = false

	tests := []struct {
		name    string
		podList *apicorev1.PodList
		want    shared.State
	}{
		{
			name: "Test Ready State",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{
							ContainerStatuses: []apicorev1.ContainerStatus{
								{
									Ready:   true,
									Started: startedTrue,
								},
							},
						},
					},
				},
			},
			want: shared.StateReady,
		},
		{
			name: "Test Processing State",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{
							ContainerStatuses: []apicorev1.ContainerStatus{
								{
									Ready:   false,
									Started: startedTrue,
								},
							},
						},
					},
				},
			},
			want: shared.StateProcessing,
		},
		{
			name: "Test Error State",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{
							ContainerStatuses: []apicorev1.ContainerStatus{
								{
									Ready:   false,
									Started: startedFalse,
								},
							},
						},
					},
				},
			},
			want: shared.StateError,
		},
		{
			name: "Test Empty State",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{},
					},
				},
			},
			want: shared.StateError,
		},
		{
			name: "Test Empty Started Condition",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{
							ContainerStatuses: []apicorev1.ContainerStatus{
								{
									Ready: false,
								},
							},
						},
					},
				},
			},
			want: shared.StateError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, statecheck.GetPodsState(tt.podList))
		})
	}
}
