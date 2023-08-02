package status

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitConditions initializes the required conditions in the Kyma CR.
func InitConditions(kyma *v1beta2.Kyma, syncEnabled bool, watcherEnabled bool) {
	kyma.Status.Conditions = []metav1.Condition{}
	for _, cond := range v1beta2.GetRequiredConditionTypes(syncEnabled, watcherEnabled) {
		kyma.UpdateCondition(cond, metav1.ConditionUnknown)
	}
}
