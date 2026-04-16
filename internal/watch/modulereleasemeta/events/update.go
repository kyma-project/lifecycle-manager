package events

import (
	"math/rand"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func HandleUpdate(
	evt event.UpdateEvent,
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
	kymaList *v1beta2.KymaList,
	updateRequeueMaxDelay time.Duration,
) {
	oldModuleReleaseMeta, ok := evt.ObjectOld.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}
	newModuleReleaseMeta, ok := evt.ObjectNew.(*v1beta2.ModuleReleaseMeta)
	if !ok {
		return
	}
	affectedModule := newModuleReleaseMeta.Spec.ModuleName
	affectedChannels := DiffModuleReleaseMetaChannels(oldModuleReleaseMeta, newModuleReleaseMeta)

	affectedKymas := GetAffectedKymas(kymaList, affectedModule, affectedChannels)
	requeueKymasWithRandomDelay(rli, affectedKymas, updateRequeueMaxDelay)
}

// DiffModuleReleaseMetaChannels returns the names of channels whose version changed or that were added/removed
// between the old and new ModuleReleaseMeta.
func DiffModuleReleaseMetaChannels(
	oldModuleReleaseMeta, newModuleReleaseMeta *v1beta2.ModuleReleaseMeta,
) []string {
	oldChannelVersions := make(map[string]string) // channel → version
	for _, channelVersion := range oldModuleReleaseMeta.Spec.Channels {
		oldChannelVersions[channelVersion.Channel] = channelVersion.Version
	}

	newChannelVersions := make(map[string]string)
	for _, channelVersion := range newModuleReleaseMeta.Spec.Channels {
		newChannelVersions[channelVersion.Channel] = channelVersion.Version
	}

	channelsDiff := findAddedOrModifiedChannels(newChannelVersions, oldChannelVersions)
	channelsDiff = append(channelsDiff, findDeletedChannels(newChannelVersions, oldChannelVersions)...)
	return channelsDiff
}

func findAddedOrModifiedChannels(newChannels map[string]string,
	oldChannels map[string]string,
) []string {
	addedOrModifiedChannels := []string{}
	for name, newVersion := range newChannels {
		if oldVersion, ok := oldChannels[name]; !ok || oldVersion != newVersion {
			addedOrModifiedChannels = append(addedOrModifiedChannels, name)
		}
	}
	return addedOrModifiedChannels
}

func findDeletedChannels(oldChannels map[string]string, newChannels map[string]string) []string {
	deletedChannels := []string{}
	for name := range oldChannels {
		if _, ok := newChannels[name]; !ok {
			deletedChannels = append(deletedChannels, name)
		}
	}
	return deletedChannels
}

// GetAffectedKymas determines which Kymas are affected by the update. It returns a list of Kymas that have modules
// assigned to the updated channels.
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

func getModuleChannel(moduleChannel string, kymaChannel string) string {
	if moduleChannel == "" {
		moduleChannel = kymaChannel
	}
	return moduleChannel
}

// requeueKymasWithRandomDelay enqueues each Kyma with a uniformly random delay in [0, maxDelay].
// When maxDelay is zero the items are added immediately (same behaviour as requeueKymas).
func requeueKymasWithRandomDelay(
	rli workqueue.TypedRateLimitingInterface[reconcile.Request],
	kymas []*types.NamespacedName,
	maxDelay time.Duration,
) {
	for _, kyma := range kymas {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      kyma.Name,
				Namespace: kyma.Namespace,
			},
		}
		if maxDelay <= 0 {
			rli.Add(req)
			continue
		}
		delay := time.Duration(rand.Int63n(int64(maxDelay))) //nolint:gosec // non-cryptographic jitter
		rli.AddAfter(req, delay)
	}
}
