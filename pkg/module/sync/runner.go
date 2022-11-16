package sync

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
)

type Runner interface {
	// Sync takes care of interpreting the given modules to acquire a new desired state for the kyma.
	Sync(
		ctx context.Context,
		kyma *v1alpha1.Kyma,
		modules common.Modules,
	) (bool, error)

	SyncModuleStatus(
		ctx context.Context,
		kyma *v1alpha1.Kyma,
		modules common.Modules,
	) bool
}
