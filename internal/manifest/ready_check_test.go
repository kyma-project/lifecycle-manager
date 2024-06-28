package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
)

func Test_getPodsState(t *testing.T) {
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
			require.Equal(t, tt.want, manifest.GetPodsState(tt.podList))
		})
	}
}

func Test_IsDeploymentReady(t *testing.T) {
	tests := []struct {
		name     string
		deploy   *apiappsv1.Deployment
		expected bool
	}{
		{
			name: "Test Deployment Ready",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionTrue,
						},
					},
					ReadyReplicas: 1,
				},
				Spec: apiappsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
				},
			},
			expected: true,
		},
		{
			name: "Test Deployment Ready using Conditions",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					ReadyReplicas: 1,
				},
				Spec: apiappsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
				},
			},
			expected: true,
		},
		{
			name: "Test Deployment Not Ready",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionFalse,
						},
					},
					ReadyReplicas: 0,
				},
				Spec: apiappsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, manifest.IsDeploymentReady(tt.deploy))
		})
	}
}
