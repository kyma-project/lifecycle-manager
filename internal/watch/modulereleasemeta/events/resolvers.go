package events

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// RegularResolver selects Kymas affected by a non-mandatory ModuleReleaseMeta event,
// filtered by the module name and the channels whose version actually changed.
type RegularResolver struct{}

func (RegularResolver) OnCreate(_ *v1beta2.ModuleReleaseMeta, _ *v1beta2.KymaList) []*types.NamespacedName {
	// A newly created MRM is not referenced by any Kyma yet.
	return nil
}

func (RegularResolver) OnUpdate(oldMRM, newMRM *v1beta2.ModuleReleaseMeta,
	kymas *v1beta2.KymaList,
) []*types.NamespacedName {
	return AffectedKymasOnUpdate(oldMRM, newMRM, kymas)
}

func (RegularResolver) OnDelete(mrm *v1beta2.ModuleReleaseMeta, kymas *v1beta2.KymaList) []*types.NamespacedName {
	return AffectedKymasOnDelete(mrm, kymas)
}

// MandatoryResolver selects all managed Kymas for a mandatory ModuleReleaseMeta event,
// since mandatory modules apply to every Kyma regardless of channel.
type MandatoryResolver struct{}

func (MandatoryResolver) OnCreate(mrm *v1beta2.ModuleReleaseMeta,
	kymas *v1beta2.KymaList,
) []*types.NamespacedName {
	return mandatoryAffectedKymas(mrm, kymas)
}

func (MandatoryResolver) OnUpdate(_, newMRM *v1beta2.ModuleReleaseMeta,
	kymas *v1beta2.KymaList,
) []*types.NamespacedName {
	return mandatoryAffectedKymas(newMRM, kymas)
}

func (MandatoryResolver) OnDelete(mrm *v1beta2.ModuleReleaseMeta,
	kymas *v1beta2.KymaList,
) []*types.NamespacedName {
	return mandatoryAffectedKymas(mrm, kymas)
}

func mandatoryAffectedKymas(mrm *v1beta2.ModuleReleaseMeta, kymas *v1beta2.KymaList) []*types.NamespacedName {
	if mrm.Spec.Mandatory == nil {
		return nil
	}

	affected := make([]*types.NamespacedName, 0, len(kymas.Items))
	for _, kyma := range kymas.Items {
		affected = append(affected, &types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()})
	}
	return affected
}
