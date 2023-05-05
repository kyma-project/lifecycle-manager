package status

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
)

// InitConditions initializes the required conditions in the Kyma CR and removes deprecated Conditions.
func InitConditions(kyma *v1beta2.Kyma, syncEnabled, watcherEnabled bool) {
	removeDeprecatedConditions(kyma)
	for _, cond := range v1beta2.GetRequiredConditionTypes(syncEnabled, watcherEnabled) {
		kyma.UpdateCondition(cond, metav1.ConditionFalse)
	}
}

func removeDeprecatedConditions(kyma *v1beta2.Kyma) {
	// validConditionTypes is a slice of all conditions allowed in a Kyma CR.
	// All other conditions will be deprecated, i.e. removed from the CR.
	validConditionTypes := []string{
		string(v1beta2.ConditionTypeModules),
		string(v1beta2.ConditionTypeModuleCatalog),
		string(v1beta2.ConditionTypeSKRWebhook),
	}

	var filteredConditions []metav1.Condition
	for _, condition := range kyma.Status.Conditions {
		if slices.Contains(validConditionTypes, condition.Type) {
			filteredConditions = append(filteredConditions, condition)
		}
	}
	kyma.Status.Conditions = filteredConditions
}
