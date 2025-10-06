package watch

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ModuleReleaseMetaEventHandler = TypedModuleReleaseMetaEventHandler[client.Object, reconcile.Request]

// TypedModuleReleaseMetaEventHandler implements handler.EventHandler.
type TypedModuleReleaseMetaEventHandler[object any, request comparable] struct {
	client.Reader

	CreateFunc func(context.Context, event.TypedCreateEvent[object], workqueue.TypedRateLimitingInterface[request])

	UpdateFunc func(context.Context, event.TypedUpdateEvent[object], workqueue.TypedRateLimitingInterface[request])

	DeleteFunc func(context.Context, event.TypedDeleteEvent[object], workqueue.TypedRateLimitingInterface[request])

	GenericFunc func(context.Context, event.TypedGenericEvent[object], workqueue.TypedRateLimitingInterface[request])
}

func NewModuleReleaseMetaEventHandler(handlerClient ChangeHandlerClient) *ModuleReleaseMetaEventHandler {
	return &ModuleReleaseMetaEventHandler{Reader: handlerClient}
}

// Create handles Create events.
func (m TypedModuleReleaseMetaEventHandler[object, request]) Create(ctx context.Context, event event.CreateEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	handleEvent(ctx, event, rli, m.Reader)
}

// Delete handles Delete events.
func (m TypedModuleReleaseMetaEventHandler[object, request]) Delete(ctx context.Context, event event.DeleteEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	handleEvent(ctx, event, rli, m.Reader)
}

// Generic handles generic events.
func (m TypedModuleReleaseMetaEventHandler[object, request]) Generic(ctx context.Context, event event.GenericEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	handleEvent(ctx, event, rli, m.Reader)
}

// Update handles Update events.
func (m TypedModuleReleaseMetaEventHandler[object, request]) Update(ctx context.Context, event event.UpdateEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	kymaList, err := getKymaList(ctx, m.Reader)
	if err != nil {
		return
	}

	oldModuleReleaseMeta, ok := event.ObjectOld.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}
	newModuleReleaseMeta, ok := event.ObjectNew.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}
	diff := DiffModuleReleaseMetaChannels(oldModuleReleaseMeta, newModuleReleaseMeta)

	affectedKymas := GetAffectedKymas(kymaList, newModuleReleaseMeta.Spec.ModuleName, diff)

	requeueKymas(rli, affectedKymas)
}

// DiffModuleReleaseMetaChannels determines the difference between the old and new ModuleReleaseMeta channels.
// It returns a map of the channels that have been updated or added.
func DiffModuleReleaseMetaChannels(
	oldModuleReleaseMeta, newModuleReleaseMeta *v1beta2.ModuleReleaseMeta,
) map[string]v1beta2.ChannelVersionAssignment {
	diff := make(map[string]v1beta2.ChannelVersionAssignment)

	oldChannels := make(map[string]v1beta2.ChannelVersionAssignment)
	for _, channel := range oldModuleReleaseMeta.Spec.Channels {
		oldChannels[channel.Channel] = channel
	}

	newChannels := make(map[string]v1beta2.ChannelVersionAssignment)
	for _, newChannel := range newModuleReleaseMeta.Spec.Channels {
		newChannels[newChannel.Channel] = newChannel
		oldChannel, ok := oldChannels[newChannel.Channel]
		if !ok || oldChannel.Version != newChannel.Version {
			diff[newChannel.Channel] = newChannel
		}
	}

	for oldChannelName, oldChannel := range oldChannels {
		if _, ok := newChannels[oldChannelName]; !ok {
			diff[oldChannelName] = oldChannel
		}
	}

	return diff
}

// GetAffectedKymas determines which Kymas are affected by the update. It returns a list of Kymas that have modules
// assigned to the updated channels.
func GetAffectedKymas(kymas *v1beta2.KymaList, moduleName string,
	newChannelAssignments map[string]v1beta2.ChannelVersionAssignment,
) []*types.NamespacedName {
	affectedKymas := make([]*types.NamespacedName, 0)
	for _, kyma := range kymas.Items {
		for _, module := range kyma.Status.Modules {
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

func handleEvent(
	ctx context.Context,
	evt interface{},
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
	reader client.Reader,
) {
	kymaList, err := getKymaList(ctx, reader)
	if err != nil {
		return
	}

	var moduleReleaseMeta *v1beta2.ModuleReleaseMeta
	var ok bool
	switch eventInstance := evt.(type) {
	case event.CreateEvent:
		moduleReleaseMeta, ok = eventInstance.Object.(*v1beta2.ModuleReleaseMeta)
	case event.DeleteEvent:
		moduleReleaseMeta, ok = eventInstance.Object.(*v1beta2.ModuleReleaseMeta)
	case event.GenericEvent:
		moduleReleaseMeta, ok = eventInstance.Object.(*v1beta2.ModuleReleaseMeta)
	default:
		return
	}

	if !ok {
		return
	}

	channelAssignment := getChannelAssignmentMapping(moduleReleaseMeta)
	affectedKymas := GetAffectedKymas(kymaList, moduleReleaseMeta.Spec.ModuleName, channelAssignment)

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

func getChannelAssignmentMapping(
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) map[string]v1beta2.ChannelVersionAssignment {
	channelMapping := make(map[string]v1beta2.ChannelVersionAssignment)
	for _, channelAssignment := range moduleReleaseMeta.Spec.Channels {
		channelMapping[channelAssignment.Channel] = channelAssignment
	}
	return channelMapping
}
