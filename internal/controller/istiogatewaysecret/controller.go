package istiogatewaysecret

import (
	"context"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

var ErrSecretNotFound = errors.New("root secret not found")

type (
	GetterFunc func(ctx context.Context, name types.NamespacedName) (*apicorev1.Secret, error)
	Handler    interface {
		ManageGatewaySecret(ctx context.Context, secret *apicorev1.Secret) error
	}
)

type Reconciler struct {
	getRootSecret GetterFunc
	handler       Handler
	intervals     queue.RequeueIntervals
}

func NewReconciler(getSecretFunc GetterFunc, handler Handler, intervals queue.RequeueIntervals) *Reconciler {
	return &Reconciler{
		getRootSecret: getSecretFunc,
		handler:       handler,
		intervals:     intervals,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logf.FromContext(ctx).V(log.DebugLevel).Info("reconcile istio gateway secret")

	rootSecret, err := r.getRootSecret(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get istio gateway root secret: %w", err)
	}
	if rootSecret == nil {
		return ctrl.Result{RequeueAfter: r.intervals.Error}, ErrSecretNotFound
	}

	err = r.handler.ManageGatewaySecret(ctx, rootSecret)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.intervals.Error},
			fmt.Errorf("failed to manage gateway secret: %w", err)
	}

	return ctrl.Result{RequeueAfter: r.intervals.Success}, nil
}
