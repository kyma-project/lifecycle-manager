package purge

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const controllerName = "purge"

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Kyma{}).
		Named(controllerName).
		WithOptions(opts).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to setup manager for purge controller: %w", err)
	}
	return nil
}
