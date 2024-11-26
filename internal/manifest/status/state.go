package status

import (
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const waitingForResourcesMsg = "waiting for resources to become ready"

var ErrInstallationConditionRequiresUpdate = errors.New("installation condition needs an update")

func SetManifestState(manifest *v1beta2.Manifest, newState shared.State) error {
	status := manifest.GetStatus()

	if newState == shared.StateProcessing {
		manifest.SetStatus(status.WithState(shared.StateProcessing).WithOperation(waitingForResourcesMsg))
		return ErrInstallationConditionRequiresUpdate
	}

	installationCondition := initInstallationCondition(manifest)
	if newState != status.State || !meta.IsStatusConditionTrue(status.Conditions, installationCondition.Type) {
		installationCondition.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, installationCondition)

		manifest.SetStatus(status.WithState(newState).WithOperation(installationCondition.Message))
		return ErrInstallationConditionRequiresUpdate
	}

	return nil
}
