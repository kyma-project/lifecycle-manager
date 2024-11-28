package status

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	resourcesAreReadyMsg   = "resources are ready"
	waitingForResourcesMsg = "waiting for resources to become ready"
)

func RequireManifestStateUpdateAfterSyncResource(manifest *v1beta2.Manifest, newState shared.State) bool {
	manifestStatus := manifest.GetStatus()

	if newState != manifestStatus.State {
		if newState == shared.StateProcessing || newState == shared.StateError {
			manifest.SetStatus(manifestStatus.WithState(newState).WithOperation(waitingForResourcesMsg))
		} else {
			ConfirmInstallationCondition(manifest)
			manifest.SetStatus(manifestStatus.WithState(newState).WithOperation(resourcesAreReadyMsg))
		}
		return true
	}

	return false
}
