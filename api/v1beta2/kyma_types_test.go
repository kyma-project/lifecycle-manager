package v1beta2_test

import (
	"testing"

	. "github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKyma_DetermineState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		givenModulesState    []State
		givenConditionStatus []metav1.ConditionStatus
		want                 State
	}{
		{
			"given moduleState contains error",
			[]State{StateError, StateReady, StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			StateError,
		},
		{
			"given moduleState contains warning but no error",
			[]State{StateWarning, StateReady, StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			StateWarning,
		},
		{
			"given moduleState in ready",
			[]State{StateReady, StateReady, StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			StateReady,
		},
		{
			"given moduleState contains error and warning",
			[]State{StateError, StateWarning, StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			StateError,
		},
		{
			"given conditions are not in true status but module in ready",
			[]State{StateReady},
			[]metav1.ConditionStatus{metav1.ConditionFalse},
			StateProcessing,
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
				condition := metav1.Condition{Status: conditionStatus}
				kyma.Status.Conditions = append(kyma.Status.Conditions, condition)
			}
			if got := kyma.DetermineState(); got != testCase.want {
				t.Errorf("DetermineState() = %v, want %v", got, testCase.want)
			}
		})
	}
}
