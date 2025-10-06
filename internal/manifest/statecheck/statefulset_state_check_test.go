package statecheck_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
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
			require.Equal(t, tt.expected, statecheck.IsStatefulSetReady(tt.statefulSet))
		})
	}
}

func Test_GetPodsState(t *testing.T) {
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
									Started: ptr.To(true),
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
									Started: ptr.To(true),
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
									Started: ptr.To(false),
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
