package istio

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	return ctrl.Result{}, nil
}
