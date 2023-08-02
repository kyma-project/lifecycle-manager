//nolint:funlen
package status_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

type testCase struct {
	name                  string
	watcherEnabled        bool
	hasSyncLabel          bool
	syncLabelValueEnabled bool
}

func TestInitConditions(t *testing.T) {
	t.Parallel()
	testcases := []testCase{
		{
			name:                  "Should Init Conditions properly with Watcher & Sync Enabled",
			watcherEnabled:        true,
			hasSyncLabel:          true,
			syncLabelValueEnabled: true,
		},
		{
			name:                  "Should Init Conditions properly with Watcher & Sync Disabled",
			watcherEnabled:        false,
			hasSyncLabel:          true,
			syncLabelValueEnabled: false,
		},
		{
			name:                  "Should Init Conditions properly with Watcher Enabled & Sync Disabled",
			watcherEnabled:        true,
			hasSyncLabel:          true,
			syncLabelValueEnabled: false,
		},
		{
			name:                  "Should Init Conditions properly with Watcher Disabled & Sync Enabled",
			watcherEnabled:        false,
			hasSyncLabel:          true,
			syncLabelValueEnabled: true,
		},
		{
			name:           "Should Init Conditions properly with Watcher Enabled & missing sync label",
			watcherEnabled: true,
			hasSyncLabel:   false,
		},
		{
			name:           "Should Init Conditions properly with Watcher Disabled & missing sync label",
			watcherEnabled: false,
			hasSyncLabel:   false,
		},
	}

	for i := range testcases {
		testcase := testcases[i]
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			kymaBuilder := testutils.NewKymaBuilder().
				WithCondition(metav1.Condition{
					Type:   string(v1beta2.DeprecatedConditionTypeReady),
					Status: metav1.ConditionFalse,
					Reason: "Deprecated",
				}).
				WithCondition(metav1.Condition{
					Type:   "ThisConditionShouldBeRemoved",
					Status: metav1.ConditionFalse,
					Reason: "Deprecated",
				})

			if testcase.hasSyncLabel {
				labelValue := v1beta2.DisableLabelValue
				if testcase.syncLabelValueEnabled {
					labelValue = v1beta2.EnableLabelValue
				}
				kymaBuilder.WithLabel(v1beta2.SyncLabel, labelValue)
			}
			kyma := kymaBuilder.Build()

			status.InitConditions(&kyma, kyma.HasSyncLabelEnabled(), testcase.watcherEnabled)

			requiredConditions := v1beta2.GetRequiredConditionTypes(kyma.HasSyncLabelEnabled(), testcase.watcherEnabled)
			if !onlyRequiredKymaConditionsPresent(&kyma, requiredConditions) {
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
