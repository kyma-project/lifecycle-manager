package events

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func HandleDelete(
	evt event.DeleteEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
	kymaList *v1beta2.KymaList,
) {
	moduleReleaseMeta, ok := evt.Object.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}
	affectedModule := moduleReleaseMeta.Spec.ModuleName
	affectedChannels := moduleReleaseMeta.GetAllChannels() // all channels are affected because MRM is being deleted

	affectedKymas := GetAffectedKymas(kymaList, affectedModule, affectedChannels)
	requeueKymas(rli, affectedKymas)
}

func requeueKymas(rli workqueue.TypedRateLimitingInterface[reconcile.Request], kymas []*types.NamespacedName) {
	for _, kyma := range kymas {
		rli.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      kyma.Name,
				Namespace: kyma.Namespace,
			},
		})
	}
}
