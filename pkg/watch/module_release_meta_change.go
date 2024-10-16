package watch

import (
	"context"

	"github.com/cloudflare/cfssl/log"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ModuleReleaseMetaEventHandler implements handler.EventHandler
type ModuleReleaseMetaEventHandler struct {
	client.Reader
}

func NewModuleReleaseMetaEventHandler(handlerClient ChangeHandlerClient) *ModuleReleaseMetaEventHandler {
	return &ModuleReleaseMetaEventHandler{Reader: handlerClient}
}

func (m *ModuleReleaseMetaEventHandler) Create(ctx context.Context, e event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {

	kymaList, err := getKymaList(ctx, m.Reader)
	if err != nil {
		return
	}

	moduleReleaseMeta, ok := e.Object.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}

	channelAssignment := getChannelAssignmentMapping(moduleReleaseMeta)
	affectedKymas := getAffectedKymas(kymaList, moduleReleaseMeta.Spec.ModuleName, channelAssignment)

	for _, kyma := range affectedKymas {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      kyma.Name,
				Namespace: kyma.Namespace,
			},
		})
	}
}

// Update handles Update events and gets old and new state
func (m *ModuleReleaseMetaEventHandler) Update(ctx context.Context, e event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	log.Info("Resource updated", "Old Object", e.ObjectOld, "New Object", e.ObjectNew)

	kymaList, err := getKymaList(ctx, m.Reader)
	if err != nil {
		return
	}

	oldModuleReleaseMeta, ok := e.ObjectOld.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}
	newModuleReleaseMeta, ok := e.ObjectNew.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}

	diff := diffModuleReleaseMetaChannels(oldModuleReleaseMeta, newModuleReleaseMeta)

	affectedKymas := getAffectedKymas(kymaList, newModuleReleaseMeta.Spec.ModuleName, diff)

	for _, kyma := range affectedKymas {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      kyma.Name,
				Namespace: kyma.Namespace,
			},
		})
	}
}

// diffModuleReleaseMetaChannels determines the difference between the old and new ModuleReleaseMeta channels. It returns
// a map of the channels that have been updated or added.
func diffModuleReleaseMetaChannels(oldModuleReleaseMeta, newModuleReleaseMeta *v1beta2.ModuleReleaseMeta) map[string]v1beta2.ChannelVersionAssignment {
	diff := make(map[string]v1beta2.ChannelVersionAssignment)

	oldChannels := make(map[string]v1beta2.ChannelVersionAssignment)
	for _, channel := range oldModuleReleaseMeta.Spec.Channels {
		oldChannels[channel.Channel] = channel
	}

	for _, newChannel := range newModuleReleaseMeta.Spec.Channels {
		oldChannel, ok := oldChannels[newChannel.Channel]
		if !ok || oldChannel.Version != newChannel.Version {
			diff[newChannel.Channel] = newChannel
		}
	}

	return diff
}

// getAffectedKymas determines which Kymas are affected by the update. It returns a list of Kymas that have modules
// assigned to the updated channels.
func getAffectedKymas(kymas *v1beta2.KymaList, moduleName string,
	newChannelAssignments map[string]v1beta2.ChannelVersionAssignment) []*types.NamespacedName {
	affectedKymas := make([]*types.NamespacedName, 0)
	for _, kyma := range kymas.Items {
		for _, module := range kyma.Spec.Modules {
			if module.Name != moduleName {
				continue
			}
			moduleChannel := module.Channel
			if moduleChannel == "" {
				moduleChannel = kyma.Spec.Channel
			}
			newAssignment, ok := newChannelAssignments[moduleChannel]
			if ok && (moduleChannel == newAssignment.Channel) {
				affectedKymas = append(affectedKymas,
					&types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()})
				break
			}
		}
	}
	return affectedKymas
}

func getChannelAssignmentMapping(moduleReleaseMeta *v1beta2.ModuleReleaseMeta) map[string]v1beta2.ChannelVersionAssignment {
	channelMapping := make(map[string]v1beta2.ChannelVersionAssignment)
	for _, channelAssignment := range moduleReleaseMeta.Spec.Channels {
		channelMapping[channelAssignment.Channel] = channelAssignment
	}
	return channelMapping
}

// Delete handles Delete events
func (m *ModuleReleaseMetaEventHandler) Delete(ctx context.Context, e event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	kymaList, err := getKymaList(ctx, m.Reader)
	if err != nil {
		return
	}

	moduleReleaseMeta, ok := e.Object.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}

	channelAssignment := getChannelAssignmentMapping(moduleReleaseMeta)
	affectedKymas := getAffectedKymas(kymaList, moduleReleaseMeta.Spec.ModuleName, channelAssignment)

	for _, kyma := range affectedKymas {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      kyma.Name,
				Namespace: kyma.Namespace,
			},
		})
	}
}
