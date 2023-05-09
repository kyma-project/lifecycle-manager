package status_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

//nolint:funlen
func TestInitConditions(t *testing.T) {
	t.Parallel()
	type args struct {
		watcherEnabled bool
		syncEnabled    bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Should Init Conditions properly with Watcher & Sync Enabled",
			args: args{
				watcherEnabled: true,
				syncEnabled:    true,
			},
		},
		{
			name: "Should Init Conditions properly with Watcher & Sync Disabled",
			args: args{
				watcherEnabled: false,
				syncEnabled:    false,
			},
		},
		{
			name: "Should Init Conditions properly with Watcher Enabled & Sync Disabled",
			args: args{
				watcherEnabled: true,
				syncEnabled:    false,
			},
		},
		{
			name: "Should Init Conditions properly with Watcher Disabled & Sync Enabled",
			args: args{
				watcherEnabled: false,
				syncEnabled:    true,
			},
		},
	}
	for _, testCase := range tests {
		tcase := testCase
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			kyma := NewTestKyma("kyma")
			syncEnabledValue := v1beta2.DisableLabelValue
			if tcase.args.syncEnabled {
				syncEnabledValue = v1beta2.EnableLabelValue
			}
			kyma.Labels[v1beta2.SyncLabel] = syncEnabledValue
			kyma.Status.Conditions = append(kyma.Status.Conditions, metav1.Condition{
				Type:               string(v1beta2.DeprecatedConditionTypeReady),
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
			if !onlyRequiredKymaConditionsPresent(kyma, v1beta2.GetRequiredConditionTypes(
				tcase.args.syncEnabled, tcase.args.watcherEnabled)) {
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
