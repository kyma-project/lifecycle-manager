package status

import (
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var ErrObjectHasEmptyState = errors.New("object has an empty state")

func Initialize(manifest *v1beta2.Manifest) error {
	status := manifest.GetStatus()

	for _, condition := range []apimetav1.Condition{
		initResourcesCondition(manifest),
		initInstallationCondition(manifest),
	} {
		if meta.FindStatusCondition(status.Conditions, condition.Type) == nil {
			meta.SetStatusCondition(&status.Conditions, condition)
		}
	}

	if status.Synced == nil {
		status.Synced = []shared.Resource{}
	}

	if status.State == "" {
		manifest.SetStatus(status.WithState(shared.StateProcessing).WithErr(ErrObjectHasEmptyState))
		return ErrObjectHasEmptyState
	}

	manifest.SetStatus(status)

	return nil
}
