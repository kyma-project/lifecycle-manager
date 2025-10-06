package status_test

import (
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

type testCase struct {
	name           string
	watcherEnabled bool
}

func TestInitConditions(t *testing.T) {
	t.Parallel()
	testcases := []testCase{
		{
			name:           "Should Init Conditions properly with Watcher Enabled & missing sync label",
			watcherEnabled: true,
		},
		{
			name:           "Should Init Conditions properly with Watcher Disabled & missing sync label",
			watcherEnabled: false,
		},
	}

	for i := range testcases {
		testcase := testcases[i]
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			kymaBuilder := builder.NewKymaBuilder().
				WithCondition(apimetav1.Condition{
					Type:   string(v1beta2.DeprecatedConditionTypeReady),
					Status: apimetav1.ConditionFalse,
					Reason: "Deprecated",
				}).
				WithCondition(apimetav1.Condition{
					Type:   "ThisConditionShouldBeRemoved",
					Status: apimetav1.ConditionFalse,
					Reason: "Deprecated",
				})

			kyma := kymaBuilder.Build()

			status.InitConditions(kyma, testcase.watcherEnabled)

			requiredConditions := v1beta2.GetRequiredConditionTypes(testcase.watcherEnabled)
			if !onlyRequiredKymaConditionsPresent(kyma, requiredConditions) {
				t.Error("Incorrect Condition Initialization")
				return
			}
		})
	}
}

func onlyRequiredKymaConditionsPresent(kyma *v1beta2.Kyma, requiredConditions []v1beta2.KymaConditionType) bool {
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
