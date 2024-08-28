package readycheck_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/readycheck"
)

func Test_GetDeploymentState(t *testing.T) {
	tests := []struct {
		name     string
		deploy   *apiappsv1.Deployment
		expected shared.State
	}{
		{
			name: "Test Ready Deployment",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionTrue,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.NewRSAvailableReason,
						},
					},
				},
			},
			expected: shared.StateReady,
		},
		{
			name: "Test Processing Deployment with Available condition and NewReplicaSetCreated",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionTrue,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.NewReplicaSetReason,
						},
					},
				},
			},
			expected: shared.StateProcessing,
		},
		{
			name: "Test Processing Deployment with Available condition and FoundNewReplicaSet",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionTrue,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.FoundNewRSReason,
						},
					},
				},
			},
			expected: shared.StateProcessing,
		},
		{
			name: "Test Processing Deployment with Available condition and ReplicaSetUpdated",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionTrue,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.ReplicaSetUpdatedReason,
						},
					},
				},
			},
			expected: shared.StateProcessing,
		},
		{
			name: "Test Processing Deployment with not Available and ReplicaSetUpdated",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionFalse,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.ReplicaSetUpdatedReason,
						},
					},
				},
			},
			expected: shared.StateProcessing,
		},
		{
			name: "Test Processing Deployment with not Available and NewReplicaSetCreated",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionFalse,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.NewReplicaSetReason,
						},
					},
				},
			},
			expected: shared.StateProcessing,
		},
		{
			name: "Test Processing Deployment with not Available and FoundNewReplicaSet",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionFalse,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.FoundNewRSReason,
						},
					},
				},
			},
			expected: shared.StateProcessing,
		},
		{
			name: "Test Processing Deployment with not Available and NewReplicaSetAvailable",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionFalse,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionTrue,
							Reason: readycheck.NewRSAvailableReason,
						},
					},
				},
			},
			expected: shared.StateProcessing,
		},
		{
			name: "Test Error Deployment with Available and ProgressDeadlineExceeded",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionTrue,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionFalse,
							Reason: "ProgressDeadlineExceeded",
						},
					},
				},
			},
			expected: shared.StateError,
		},
		{
			name: "Test Error Deployment with Available and ReplicaSetCreateError",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionTrue,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionFalse,
							Reason: "ReplicaSetCreateError",
						},
					},
				},
			},
			expected: shared.StateError,
		},
		{
			name: "Test Error Deployment with not Available and ReplicaSetCreateError",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionFalse,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionFalse,
							Reason: "ReplicaSetCreateError",
						},
					},
				},
			},
			expected: shared.StateError,
		},
		{
			name: "Test Error Deployment with not Available and ProgressDeadlineExceeded",
			deploy: &apiappsv1.Deployment{
				Status: apiappsv1.DeploymentStatus{
					Conditions: []apiappsv1.DeploymentCondition{
						{
							Type:   apiappsv1.DeploymentAvailable,
							Status: apicorev1.ConditionFalse,
						},
						{
							Type:   apiappsv1.DeploymentProgressing,
							Status: apicorev1.ConditionFalse,
							Reason: "ProgressDeadlineExceeded",
						},
					},
				},
			},
			expected: shared.StateError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, readycheck.GetDeploymentState(tt.deploy))
		})
	}
}
