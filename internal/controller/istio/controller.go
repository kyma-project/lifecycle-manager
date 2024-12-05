package istio

import (
	"context"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

type Reconciler struct {
	client.Client
	queue.RequeueIntervals
	handler *gatewaysecret.Handler
}

func NewReconciler(client client.Client, requeueIntervals queue.RequeueIntervals, handler *gatewaysecret.Handler) *Reconciler {
	return &Reconciler{
		Client:           client,
		RequeueIntervals: requeueIntervals,
		handler:          handler,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("reconcile istio gateway secret")

	secret := &apicorev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get istio gateway secret: %w", err)
	}

	err := r.handler.ManageGatewaySecret(ctx, secret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to manage gateway secret: %w", err)
	}

	return ctrl.Result{}, nil
}
