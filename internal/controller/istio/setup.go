package istio

import (
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
)

const controllerName = "istio-controller"

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&apicorev1.Secret{}).
		Named(controllerName).
		WithOptions(opts).
		Complete(r); err != nil {
		return fmt.Errorf("failed to setup manager for istio controller: %w", err)
	}
	return nil

}
