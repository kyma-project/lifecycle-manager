package status

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

// InitConditions initializes the required conditions in the Kyma CR and removes deprecated Conditions.
func InitConditions(kyma *v1beta1.Kyma, watcherEnabled bool) {
	// Removing deprecated `Ready` condition
	existingCondition := &kyma.Status.Conditions
	if deprecatedCondition := apimeta.FindStatusCondition(*existingCondition,
		string(v1beta1.DeprecatedConditionTypeReady)); deprecatedCondition != nil {
		apimeta.RemoveStatusCondition(existingCondition, string(v1beta1.DeprecatedConditionTypeReady))
	}
	// Add required Conditions
	for _, cond := range v1beta1.GetRequiredConditionTypes(kyma.Spec.Sync.Enabled, watcherEnabled) {
		kyma.UpdateCondition(cond, metav1.ConditionFalse)
	}
}
