package status_test

import (
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

type testCase struct {
	name                   string
	watcherEnabled         bool
	skrImagePullSecretSync bool
}

func TestInitConditions(t *testing.T) {
	t.Parallel()
	testcases := []testCase{
		{
			name:                   "Watcher enabled, SKRImagePullSecretSync disabled",
			watcherEnabled:         true,
			skrImagePullSecretSync: false,
		},
		{
			name:                   "Both Watcher and SKRImagePullSecretSync disabled",
			watcherEnabled:         false,
			skrImagePullSecretSync: false,
		},
		{
			name:                   "Watcher disabled, SKRImagePullSecretSync enabled",
			watcherEnabled:         false,
			skrImagePullSecretSync: true,
		},
		{
			name:                   "Both Watcher and SKRImagePullSecretSync enabled",
			watcherEnabled:         true,
			skrImagePullSecretSync: true,
		},
	}

	for i := range testcases {
		testcase := testcases[i]
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			// Start with a Kyma that has some pre-existing conditions including deprecated ones
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

			// ACT
			status.InitConditions(kyma, testcase.watcherEnabled, testcase.skrImagePullSecretSync)

			// ASSERT

			// Check that all pre-existing conditions are cleared
			if len(kyma.Status.Conditions) == 0 {
				t.Error("Expected conditions to be initialized, but found none")
				return
			}

			// Verify that deprecated conditions are removed
			for _, condition := range kyma.Status.Conditions {
				if condition.Type == string(v1beta2.DeprecatedConditionTypeReady) {
					t.Error("Deprecated Ready condition should have been removed")
				}
				if condition.Type == "ThisConditionShouldBeRemoved" {
					t.Error("Custom condition should have been removed")
				}
			}

			// Check for required conditions based on the specific test case
			expectedConditions := getExpectedConditions(testcase.watcherEnabled, testcase.skrImagePullSecretSync)

			if len(kyma.Status.Conditions) != len(expectedConditions) {
				t.Errorf("Expected %d conditions, but got %d", len(expectedConditions), len(kyma.Status.Conditions))
				return
			}

			// Verify each expected condition is present with correct initial state
			for _, expectedCondition := range expectedConditions {
				found := false
				for _, actualCondition := range kyma.Status.Conditions {
					if actualCondition.Type == string(expectedCondition) {
						found = true
						if actualCondition.Status != apimetav1.ConditionUnknown {
							t.Errorf("Expected condition %s to have status Unknown, but got %s",
								expectedCondition, actualCondition.Status)
						}
						if actualCondition.Reason != string(v1beta2.ConditionReason) {
							t.Errorf("Expected condition %s to have reason %s, but got %s",
								expectedCondition, v1beta2.ConditionReason, actualCondition.Reason)
						}
						// Verify the message is generated correctly
						expectedMessage := v1beta2.GenerateMessage(expectedCondition, apimetav1.ConditionUnknown)
						if actualCondition.Message != expectedMessage {
							t.Errorf("Expected condition %s to have message %q, but got %q",
								expectedCondition, expectedMessage, actualCondition.Message)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected condition %s was not found", expectedCondition)
				}
			}

			// Verify no unexpected conditions are present
			for _, actualCondition := range kyma.Status.Conditions {
				found := false
				for _, expectedCondition := range expectedConditions {
					if actualCondition.Type == string(expectedCondition) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unexpected condition found: %s", actualCondition.Type)
				}
			}
		})
	}
}

func TestInitConditions_WithEmptyKyma(t *testing.T) {
	t.Parallel()

	kyma := builder.NewKymaBuilder().Build()

	// ACT
	status.InitConditions(kyma, false, false)

	// ASSERT

	// Should have exactly 2 conditions (Modules and ModuleCatalog)
	if len(kyma.Status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, but got %d", len(kyma.Status.Conditions))
	}

	// Verify specific conditions are present
	hasModules := false
	hasModuleCatalog := false
	for _, condition := range kyma.Status.Conditions {
		switch condition.Type {
		case string(v1beta2.ConditionTypeModules):
			hasModules = true
		case string(v1beta2.ConditionTypeModuleCatalog):
			hasModuleCatalog = true
		}
	}

	if !hasModules {
		t.Error("Missing ConditionTypeModules")
	}
	if !hasModuleCatalog {
		t.Error("Missing ConditionTypeModuleCatalog")
	}
}

func TestInitConditions_MessageGeneration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                   string
		watcherEnabled         bool
		skrImagePullSecretSync bool
		expectedMessages       map[v1beta2.KymaConditionType]string
	}{
		{
			name:                   "Basic conditions with unknown state",
			watcherEnabled:         false,
			skrImagePullSecretSync: false,
			expectedMessages: map[v1beta2.KymaConditionType]string{
				v1beta2.ConditionTypeModules:       "modules state is unknown",
				v1beta2.ConditionTypeModuleCatalog: "module templates synchronization state is unknown",
			},
		},
		{
			name:                   "All conditions enabled with unknown state",
			watcherEnabled:         true,
			skrImagePullSecretSync: true,
			expectedMessages: map[v1beta2.KymaConditionType]string{
				v1beta2.ConditionTypeModules:                "modules state is unknown",
				v1beta2.ConditionTypeModuleCatalog:          "module templates synchronization state is unknown",
				v1beta2.ConditionTypeSKRWebhook:             "skrwebhook is out of sync and needs to be resynchronized",
				v1beta2.ConditionTypeSKRImagePullSecretSync: "skr image pull secret is out of sync and needs to be resynchronized",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kyma := builder.NewKymaBuilder().Build()
			status.InitConditions(kyma, tc.watcherEnabled, tc.skrImagePullSecretSync)

			for conditionType, expectedMessage := range tc.expectedMessages {
				found := false
				for _, condition := range kyma.Status.Conditions {
					if condition.Type == string(conditionType) {
						found = true
						if condition.Message != expectedMessage {
							t.Errorf("Condition %s: expected message %q, got %q",
								conditionType, expectedMessage, condition.Message)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected condition %s not found", conditionType)
				}
			}
		})
	}
}

func TestInitConditions_RemovesDeprecatedConditions(t *testing.T) {
	t.Parallel()

	// Create Kyma with deprecated condition
	kyma := builder.NewKymaBuilder().
		WithCondition(apimetav1.Condition{
			Type:   string(v1beta2.DeprecatedConditionTypeReady),
			Status: apimetav1.ConditionTrue,
			Reason: "Test",
		}).
		WithCondition(apimetav1.Condition{
			Type:   "SomeOldCondition",
			Status: apimetav1.ConditionTrue,
			Reason: "Test",
		}).
		Build()

	// Verify pre-existing conditions
	if len(kyma.Status.Conditions) != 2 {
		t.Fatalf("Expected 2 pre-existing conditions, got %d", len(kyma.Status.Conditions))
	}

	status.InitConditions(kyma, true, true)

	// Verify deprecated conditions are removed
	for _, condition := range kyma.Status.Conditions {
		if condition.Type == string(v1beta2.DeprecatedConditionTypeReady) {
			t.Error("DeprecatedConditionTypeReady should have been removed")
		}
		if condition.Type == "SomeOldCondition" {
			t.Error("SomeOldCondition should have been removed")
		}
	}

	// Should have 4 new conditions
	expectedConditions := 4 // Modules, ModuleCatalog, SKRWebhook, SKRImagePullSecretSync
	if len(kyma.Status.Conditions) != expectedConditions {
		t.Errorf("Expected %d conditions after init, got %d", expectedConditions, len(kyma.Status.Conditions))
	}
}

func TestInitConditions_WhenCalledTwice_ResultEnsuresIdempotency(t *testing.T) {
	t.Parallel()

	kyma := builder.NewKymaBuilder().Build()

	// ACT
	status.InitConditions(kyma, true, false)
	firstCallConditions := make([]apimetav1.Condition, len(kyma.Status.Conditions))
	copy(firstCallConditions, kyma.Status.Conditions)

	status.InitConditions(kyma, true, false)
	secondCallConditions := kyma.Status.Conditions

	// ASSERT

	// Should have same number of conditions
	if len(firstCallConditions) != len(secondCallConditions) {
		t.Errorf("Expected same number of conditions, got %d vs %d",
			len(firstCallConditions), len(secondCallConditions))
	}

	// Should have same condition types
	for _, firstCondition := range firstCallConditions {
		found := false
		for _, secondCondition := range secondCallConditions {
			if firstCondition.Type == secondCondition.Type {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Condition %s missing after second call", firstCondition.Type)
		}
	}
}

func TestInitConditions_Metadata(t *testing.T) {
	t.Parallel()

	// Create Kyma with specific generation
	kyma := builder.NewKymaBuilder().
		WithGeneration(5).
		Build()

	// ACT
	status.InitConditions(kyma, false, false)

	// ASSERT

	// Verify all conditions have correct metadata
	for _, condition := range kyma.Status.Conditions {
		if condition.ObservedGeneration != kyma.GetGeneration() {
			t.Errorf("Condition %s has ObservedGeneration %d, expected %d",
				condition.Type, condition.ObservedGeneration, kyma.GetGeneration())
		}

		if condition.Reason != string(v1beta2.ConditionReason) {
			t.Errorf("Condition %s has Reason %s, expected %s",
				condition.Type, condition.Reason, v1beta2.ConditionReason)
		}

		if condition.Status != apimetav1.ConditionUnknown {
			t.Errorf("Condition %s has Status %s, expected %s",
				condition.Type, condition.Status, apimetav1.ConditionUnknown)
		}

		if condition.Message == "" {
			t.Errorf("Condition %s has empty Message", condition.Type)
		}

		if condition.LastTransitionTime.IsZero() {
			t.Errorf("Condition %s has zero LastTransitionTime", condition.Type)
		}
	}
}

// getExpectedConditions returns the specific conditions that should be present
// for the given configuration, avoiding circular dependency on GetRequiredConditionTypes
func getExpectedConditions(watcherEnabled, skrImagePullSecretSync bool) []v1beta2.KymaConditionType {
	// Based on the logic in condition_messages.go, these are the expected conditions:
	expectedConditions := []v1beta2.KymaConditionType{
		v1beta2.ConditionTypeModules,       // Always required
		v1beta2.ConditionTypeModuleCatalog, // Always required
	}

	if watcherEnabled {
		expectedConditions = append(expectedConditions, v1beta2.ConditionTypeSKRWebhook)
	}

	if skrImagePullSecretSync {
		expectedConditions = append(expectedConditions, v1beta2.ConditionTypeSKRImagePullSecretSync)
	}

	return expectedConditions
}
