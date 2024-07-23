package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/lifecycle-manager/internal/manifest"
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
			require.Equal(t, tt.expected, manifest.IsStatefulSetReady(tt.statefulSet))
		})
	}
}
