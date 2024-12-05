package istio

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	controllerName    = "istio-controller"
	kcpRootSecretName = "klm-watcher" //nolint:gosec // gatewaySecretName is not a credential
	istioNamespace    = "istio-system"
)

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	secretPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isRootSecret(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isRootSecret(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isRootSecret(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isRootSecret(e.Object)
		},
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&apicorev1.Secret{}).
		Named(controllerName).
		WithOptions(opts).
		WithEventFilter(secretPredicate).
		Complete(r); err != nil {
		return fmt.Errorf("failed to setup manager for istio controller: %w", err)
	}

	return nil
}

func isRootSecret(object client.Object) bool {
	return object.GetNamespace() == istioNamespace && object.GetName() == kcpRootSecretName

}
