package status

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var ErrObjectHasEmptyState = errors.New("object has an empty state")

func Initialize(manifest *v1beta2.Manifest) error {
	InitializeStatusConditions(manifest)

	status := manifest.GetStatus()

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
