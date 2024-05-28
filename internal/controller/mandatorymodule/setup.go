package mandatorymodule

import (
	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watch"
)

const (
	installationControllerName = "mandatory-module-installation"
	deletionControllerName     = "mandatory-module-deletion"
)

func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	predicates := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Kyma{}).
		Named(installationControllerName).
		WithOptions(opts).
		WithEventFilter(predicates).
		Watches(
			&v1beta2.ModuleTemplate{},
			handler.EnqueueRequestsFromMapFunc(watch.NewMandatoryTemplateChangeHandler(r).Watch()),
			builder.WithPredicates(predicates),
		).
		Watches(&apicorev1.Secret{}, handler.Funcs{}).
		Complete(r)
}

func (r *DeletionReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.ModuleTemplate{}).
		Named(deletionControllerName).
		WithOptions(opts).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
