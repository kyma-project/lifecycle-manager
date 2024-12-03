package status_test

import (
	"testing"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/status"
)

func TestConfirmInstallationCondition(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetGeneration(1)

	status.ConfirmInstallationCondition(manifest)

	conditions := manifest.GetStatus().Conditions
	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}

	condition := conditions[0]
	if condition.Type != string(status.ConditionTypeInstallation) {
		t.Errorf("expected condition type %s, got %s", status.ConditionTypeInstallation, condition.Type)
	}
	if condition.Reason != string(status.ConditionReasonReady) {
		t.Errorf("expected condition reason %s, got %s", status.ConditionReasonReady, condition.Reason)
	}
	if condition.Status != apimetav1.ConditionTrue {
		t.Errorf("expected condition status %s, got %s", apimetav1.ConditionTrue, condition.Status)
	}
	if condition.Message != "installation is ready and resources can be used" {
		t.Errorf("expected condition message %s, got %s", "installation is ready and resources can be used",
			condition.Message)
	}
	if condition.ObservedGeneration != manifest.GetGeneration() {
		t.Errorf("expected observed generation %d, got %d", manifest.GetGeneration(), condition.ObservedGeneration)
	}
}
