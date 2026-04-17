package events

import (
	"slices"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// AffectedKymasOnDelete returns the Kymas that should be requeued when an MRM is deleted.
// All channels are affected because the entire MRM is going away.
func AffectedKymasOnDelete(mrm *v1beta2.ModuleReleaseMeta, kymaList *v1beta2.KymaList) []*types.NamespacedName {
	return GetAffectedKymas(kymaList, mrm.Spec.ModuleName, mrm.GetAllChannels())
}

// GetAffectedKymas determines which Kymas are affected by the update. It returns a list of Kymas that have the affected
// modules installed from the affected channels.
func GetAffectedKymas(
	kymaList *v1beta2.KymaList,
	affectedModule string,
	affectedChannels []string,
) []*types.NamespacedName {
	affectedKymas := make([]*types.NamespacedName, 0)
	for _, kyma := range kymaList.Items {
		for _, module := range kyma.Status.Modules {
			if module.Name != affectedModule {
				continue
			}
			moduleChannel := getModuleChannel(module.Channel, kyma.Spec.Channel)
			if slices.Contains(affectedChannels, moduleChannel) {
				affectedKymas = append(affectedKymas,
					&types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()})
				break
			}
		}
	}
	return affectedKymas
}

func getModuleChannel(moduleChannel, kymaChannel string) string {
	if moduleChannel == "" {
		return kymaChannel
	}
	return moduleChannel
}
