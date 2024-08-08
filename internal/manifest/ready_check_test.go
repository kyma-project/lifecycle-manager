package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/require"
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
