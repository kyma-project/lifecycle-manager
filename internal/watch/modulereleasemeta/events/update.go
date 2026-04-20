package events

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// AffectedKymasOnUpdate returns the Kymas that should be requeued when an MRM is updated.
func AffectedKymasOnUpdate(oldMRM, newMRM *v1beta2.ModuleReleaseMeta,
	kymaList *v1beta2.KymaList,
) []*types.NamespacedName {
	affectedChannels := diffModuleReleaseMetaChannels(oldMRM, newMRM)
	return getAffectedKymas(kymaList, newMRM.Spec.ModuleName, affectedChannels)
}

func diffModuleReleaseMetaChannels(
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
	channelsDiff = append(channelsDiff, findDeletedChannels(oldChannelVersions, newChannelVersions)...)
	return channelsDiff
}

func findAddedOrModifiedChannels(newChannels, oldChannels map[string]string) []string {
	addedOrModified := []string{}
	for name, newVersion := range newChannels {
		if oldVersion, ok := oldChannels[name]; !ok || oldVersion != newVersion {
			addedOrModified = append(addedOrModified, name)
		}
	}
	return addedOrModified
}

func findDeletedChannels(oldChannels, newChannels map[string]string) []string {
	deleted := []string{}
	for name := range oldChannels {
		if _, ok := newChannels[name]; !ok {
			deleted = append(deleted, name)
		}
	}
	return deleted
}
