package status

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetConditions checks if all required conditions do exists and removes deprecated Conditions.
// If one or many Conditions are missing, they will be created and `true` will be returned
// to determine that an update is needed.
// If all required conditions exist, they will be set to `False`.
func SetConditions(kyma *v1beta1.Kyma, watcherEnabled bool) bool {
	conditionUpdateNeeded := false

	existingConditions := kyma.Status.Conditions
	// Remove deprecated `Ready` condition
	if deprecatedCondition := apimeta.FindStatusCondition(existingConditions, string(v1beta1.DeprecatedConditionReady)); deprecatedCondition != nil {
		apimeta.RemoveStatusCondition(&existingConditions, string(v1beta1.DeprecatedConditionReady))
		conditionUpdateNeeded = true
	}
	// Add required Conditions
	for _, cond := range v1beta1.GetRequiredConditions(kyma.Spec.Sync.Enabled, watcherEnabled) {
		if existingCondition := apimeta.FindStatusCondition(existingConditions, string(cond)); existingCondition == nil {
			// Condition does not exist, KymaCR needs to be patched after Condition is being added
			conditionUpdateNeeded = true
		}
		kyma.UpdateCondition(cond, metav1.ConditionFalse)
	}
	return conditionUpdateNeeded
}
