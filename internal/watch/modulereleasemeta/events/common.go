package events

import (
	"slices"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func getAffectedKymas(
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
