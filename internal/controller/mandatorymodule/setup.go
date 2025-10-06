package mandatorymodule

import (
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
)

const (
	installationControllerName = "mandatory-module-installation"
	deletionControllerName     = "mandatory-module-deletion"
)

func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Kyma{}).
		Named(installationControllerName).
		WithOptions(opts).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})).
		Watches(
			&v1beta2.ModuleTemplate{},
			handler.EnqueueRequestsFromMapFunc(watch.NewMandatoryTemplateChangeHandler(r).Watch()),
		).
		Watches(&apicorev1.Secret{}, handler.Funcs{}).
		Complete(r); err != nil {
		return fmt.Errorf("failed to setup manager for mandatory module installation controller: %w", err)
	}
	return nil
}

func (r *DeletionReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.ModuleTemplate{}).
		Named(deletionControllerName).
		WithOptions(opts).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})).
		Complete(r); err != nil {
		return fmt.Errorf("failed to setup manager for mandatory module deletion controller: %w", err)
	}
	return nil
}
