package status_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/pkg/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func TestInitConditions(t *testing.T) {
	t.Parallel()
	type args struct {
		watcherEnabled bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Should Init Conditions properly with Watcher Enabled",
			args: args{
				watcherEnabled: true,
			},
		},
		{
			name: "Should Init Conditions properly with Watcher Disabled",
			args: args{
				watcherEnabled: false,
			},
		},
	}
	for _, testCase := range tests {
		tcase := testCase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			kyma := NewTestKyma("kyma")
			kyma.Status.Conditions = append(kyma.Status.Conditions, metav1.Condition{
				Type:               string(v1beta1.DeprecatedConditionTypeReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: kyma.GetGeneration(),
				Reason:             "Deprecated",
			})
			kyma.Status.Conditions = append(kyma.Status.Conditions, metav1.Condition{
				Type:               "ThisConditionShouldBeRemoved",
				Status:             metav1.ConditionFalse,
				ObservedGeneration: kyma.GetGeneration(),
				Reason:             "Deprecated",
			})

			status.InitConditions(kyma, tcase.args.watcherEnabled)

			if !onlyRequiredKymaConditionsPresent(kyma, v1beta1.GetRequiredConditionTypes(
				false, tcase.args.watcherEnabled)) {
				t.Error("Incorrect Condition Initialization")
				return
			}
		})
	}
}

func onlyRequiredKymaConditionsPresent(kyma *v1beta1.Kyma, requiredConditions []v1beta1.KymaConditionType) bool {
	if len(kyma.Status.Conditions) != len(requiredConditions) {
		return false
	}

	for _, conditionType := range requiredConditions {
		exists := false
		for _, kymaCondition := range kyma.Status.Conditions {
			if kymaCondition.Type == string(conditionType) {
				exists = true
				break
			}
		}
		if !exists {
			return false
		}
	}
	return true
}
