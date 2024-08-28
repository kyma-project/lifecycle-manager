package readycheck_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/readycheck"
)

func Test_IsStatefulSetReady(t *testing.T) {
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
					Replicas: ptr.To(int32(1)),
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
					Replicas: ptr.To(int32(1)),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, readycheck.IsStatefulSetReady(tt.statefulSet))
		})
	}
}

func Test_getPodsState(t *testing.T) {
	tests := []struct {
		name    string
		podList *apicorev1.PodList
		want    shared.State
	}{
		{
			name: "Test Processing State",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{
							ContainerStatuses: []apicorev1.ContainerStatus{
								{
									State: apicorev1.ContainerState{
										Waiting: &apicorev1.ContainerStateWaiting{},
									},
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
									State: apicorev1.ContainerState{
										Terminated: &apicorev1.ContainerStateTerminated{
											ExitCode: 1,
										},
									},
								},
							},
						},
					},
				},
			},
			want: shared.StateError,
		},
		{
			name: "Test Ready State",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{
							ContainerStatuses: []apicorev1.ContainerStatus{
								{
									Ready: true,
									State: apicorev1.ContainerState{
										Terminated: &apicorev1.ContainerStateTerminated{
											ExitCode: 0,
										},
									},
								},
							},
						},
					},
				},
			},
			want: shared.StateReady,
		},
		{
			name: "Test Processing State with Ready set to false",
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
			want: shared.StateProcessing,
		},
		{
			name: "Test Ready State with Ready set to true",
			podList: &apicorev1.PodList{
				Items: []apicorev1.Pod{
					{
						Status: apicorev1.PodStatus{
							ContainerStatuses: []apicorev1.ContainerStatus{
								{
									Ready: true,
								},
							},
						},
					},
				},
			},
			want: shared.StateReady,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, readycheck.GetPodsState(tt.podList))
		})
	}
}
