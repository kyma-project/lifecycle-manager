package statecheck_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
)

func TestManagerStateCheck_GetState(t *testing.T) {
	tests := []struct {
		name          string
		resources     []*resource.Info
		isDeployment  bool
		isStateFulSet bool
		expectedError error
	}{
		{
			name: "Test Deployment State Checker",
			resources: []*resource.Info{
				{
					Object: &apiappsv1.Deployment{
						ObjectMeta: apimetav1.ObjectMeta{Name: "some-deploy"},
					},
				},
			},
			isDeployment:  true,
			isStateFulSet: false,
			expectedError: nil,
		},
		{
			name: "Test StatefulSet State Checker",
			resources: []*resource.Info{
				{
					Object: &apiappsv1.StatefulSet{
						ObjectMeta: apimetav1.ObjectMeta{Name: "some-statefulset"},
					},
				},
			},
			isDeployment:  false,
			isStateFulSet: true,
			expectedError: nil,
		},
		{
			name:          "Test no manager found",
			resources:     []*resource.Info{},
			isDeployment:  false,
			isStateFulSet: false,
			expectedError: nil,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			scheme := machineryruntime.NewScheme()
			_ = apiappsv1.AddToScheme(scheme)
			clnt := fake.NewClientBuilder().WithScheme(scheme).Build()

			statefulsetChecker := &StatefulSetStateCheckerStub{}
			deploymentChecker := &DeploymentStateCheckerStub{}
			m := statecheck.NewManagerStateCheck(statefulsetChecker, deploymentChecker)
			got, err := m.GetState(t.Context(), clnt, testCase.resources)

			if testCase.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.Equal(t, testCase.expectedError, err)
			}

			if testCase.isDeployment {
				require.True(t, deploymentChecker.called)
				require.False(t, statefulsetChecker.called)
				require.Equal(t, shared.StateProcessing, got)
			}

			if testCase.isStateFulSet {
				require.True(t, statefulsetChecker.called)
				require.False(t, deploymentChecker.called)
				require.Equal(t, shared.StateReady, got)
			}
		})
	}
}

// Test Stubs.
type DeploymentStateCheckerStub struct {
	called bool
}

func (d *DeploymentStateCheckerStub) GetState(_ *apiappsv1.Deployment) (shared.State, error) {
	d.called = true
	return shared.StateProcessing, nil
}

type StatefulSetStateCheckerStub struct {
	called bool
}

func (s *StatefulSetStateCheckerStub) GetState(_ context.Context, _ client.Client,
	_ *apiappsv1.StatefulSet,
) (shared.State, error) {
	s.called = true
	return shared.StateReady, nil
}
