package istio

import (
	"context"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

type Reconciler struct {
	client.Client
}

func NewReconciler(client client.Client) *Reconciler {
	return &Reconciler{
		Client: client,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("reconcile istio gateway secret")

	secret := &apicorev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get istio gateway secret: %w", err)
	}

	err := gatewaysecret.ManageGatewaySecret(ctx, secret)

	return ctrl.Result{}, nil
}
