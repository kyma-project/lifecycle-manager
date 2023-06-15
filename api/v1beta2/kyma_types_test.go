package v1beta2_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKyma_DetermineState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		givenModulesState    []v1beta2.State
		givenConditionStatus []metav1.ConditionStatus
		want                 v1beta2.State
	}{
		{
			"given moduleState contains error",
			[]v1beta2.State{v1beta2.StateError, v1beta2.StateReady, v1beta2.StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			v1beta2.StateError,
		},
		{
			"given moduleState contains warning but no error",
			[]v1beta2.State{v1beta2.StateWarning, v1beta2.StateReady, v1beta2.StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			v1beta2.StateWarning,
		},
		{
			"given moduleState in ready",
			[]v1beta2.State{v1beta2.StateReady, v1beta2.StateReady, v1beta2.StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			v1beta2.StateReady,
		},
		{
			"given moduleState contains error and warning",
			[]v1beta2.State{v1beta2.StateError, v1beta2.StateWarning, v1beta2.StateReady},
			[]metav1.ConditionStatus{metav1.ConditionTrue},
			v1beta2.StateError,
		},
		{
			"given conditions are not in true status but module in ready",
			[]v1beta2.State{v1beta2.StateReady},
			[]metav1.ConditionStatus{metav1.ConditionFalse},
			v1beta2.StateProcessing,
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
