package purge

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconcileHandler struct {
	Get                            func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	handleKymaNotFoundError        func(logger logr.Logger, kyma *v1beta2.Kyma, err error) (ctrl.Result, error)
	handleKymaNotMarkedForDeletion func(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error)
	handlePurgeNotDue              func(logger logr.Logger, kyma *v1beta2.Kyma, requeueAfter time.Duration) (ctrl.Result, error)
	handleSkrNotFoundError         func(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result, error)
	handlePurge                    func(ctx context.Context, kyma *v1beta2.Kyma, remoteClient client.Client, start time.Time) (ctrl.Result, error)
	calculateRequeueAfterTime      func(kyma *v1beta2.Kyma) time.Duration

	SkrContextFactory remote.SkrContextProvider
}
