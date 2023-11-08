package v1beta2_test

import (
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestKyma_DetermineState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		givenModulesState    []shared.State
		givenConditionStatus []apimetav1.ConditionStatus
		want                 shared.State
	}{
		{
			"given moduleshared.State contains error",
			[]shared.State{shared.StateError, shared.StateReady, shared.StateReady},
			[]apimetav1.ConditionStatus{apimetav1.ConditionTrue},
			shared.StateError,
		},
		{
			"given moduleshared.State contains warning but no error",
			[]shared.State{shared.StateWarning, shared.StateReady, shared.StateReady},
			[]apimetav1.ConditionStatus{apimetav1.ConditionTrue},
			shared.StateWarning,
		},
		{
			"given moduleshared.State in ready",
			[]shared.State{shared.StateReady, shared.StateReady, shared.StateReady},
			[]apimetav1.ConditionStatus{apimetav1.ConditionTrue},
			shared.StateReady,
		},
		{
			"given moduleshared.State contains error and warning",
			[]shared.State{shared.StateError, shared.StateWarning, shared.StateReady},
			[]apimetav1.ConditionStatus{apimetav1.ConditionTrue},
			shared.StateError,
		},
		{
			"given conditions are not in true status but module in ready",
			[]shared.State{shared.StateReady},
			[]apimetav1.ConditionStatus{apimetav1.ConditionFalse},
			shared.StateProcessing,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			kyma := testutils.NewTestKyma("test-kyma")
			for _, state := range testCase.givenModulesState {
				moduleStatus := v1beta2.ModuleStatus{
					State: state,
				}
				kyma.Status.Modules = append(kyma.Status.Modules, moduleStatus)
			}
			for _, conditionStatus := range testCase.givenConditionStatus {
				condition := apimetav1.Condition{Status: conditionStatus}
				kyma.Status.Conditions = append(kyma.Status.Conditions, condition)
			}
			if got := kyma.DetermineState(); got != testCase.want {
				t.Errorf("DetermineState() = %v, want %v", got, testCase.want)
			}
		})
	}
}
