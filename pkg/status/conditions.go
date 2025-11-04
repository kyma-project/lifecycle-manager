package status

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// InitConditions initializes the required conditions in the Kyma CR.
func InitConditions(kyma *v1beta2.Kyma, watcherEnabled, skrImagePullSecretEnabled bool) {
	kyma.Status.Conditions = []apimetav1.Condition{}
	for _, cond := range v1beta2.GetRequiredConditionTypes(watcherEnabled, skrImagePullSecretEnabled) {
		kyma.UpdateCondition(cond, apimetav1.ConditionUnknown)
	}
}
