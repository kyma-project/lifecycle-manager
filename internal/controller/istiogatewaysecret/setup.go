package istiogatewaysecret

import (
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	controllerName = "istio-controller"

	// TODO move to config
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

		// TODO probably not needed
		//DeleteFunc: func(e event.DeleteEvent) bool {
		//	return isRootSecret(e.Object)
		//},
		//GenericFunc: func(e event.GenericEvent) bool {
		//	return isRootSecret(e.Object)
		//},
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
