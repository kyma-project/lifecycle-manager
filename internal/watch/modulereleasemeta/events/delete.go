package events

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// AffectedKymasOnDelete returns the Kymas that should be requeued when an MRM is deleted.
// All channels are affected because the entire MRM is going away.
func AffectedKymasOnDelete(mrm *v1beta2.ModuleReleaseMeta, kymaList *v1beta2.KymaList) []*types.NamespacedName {
	return getAffectedKymas(kymaList, mrm.Spec.ModuleName, mrm.GetAllChannels())
}
