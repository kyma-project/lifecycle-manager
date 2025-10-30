package mandatorymodule

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
)

const (
	installationControllerName = "mandatory-module-installation"
	deletionControllerName     = "mandatory-module-deletion"
)

func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	err := setupFieldIndexForMandatoryMrm(mgr)
	if err != nil {
		return fmt.Errorf("failed to setup field indexer for mandatory ModuleReleaseMetas: %w", err)
	}

	err = setupFieldIndexForModuleTemplateByModuleVersion(mgr)
	if err != nil {
		return fmt.Errorf("failed to setup field indexer for ModuleTemplates by version: %w", err)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Kyma{}).
		Named(installationControllerName).
		WithOptions(opts).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})).
		Watches(
			&v1beta2.ModuleReleaseMeta{},
			handler.EnqueueRequestsFromMapFunc(watch.NewMandatoryMrmChangeHandler(mgr.GetClient()).Watch()),
		).
		Watches(&apicorev1.Secret{}, handler.Funcs{}).
		Complete(reconcile.AsReconciler[*v1beta2.Kyma](mgr.GetClient(), r)); err != nil {
		return fmt.Errorf("failed to setup manager for mandatory module installation controller: %w", err)
	}
	return nil
}

// setupFieldIndexForMandatoryMrm sets up a field indexer on MRMs to optimize lookup for mandatory MRMs.
// MatchingFields: "is-mandatory" -> "true"/"false"
func setupFieldIndexForMandatoryMrm(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1beta2.ModuleReleaseMeta{},
		shared.MrmMandatoryModuleFieldIndexName,
		func(obj client.Object) []string {
			mrm, ok := obj.(*v1beta2.ModuleReleaseMeta)
			if !ok {
				return nil
			}
			if mrm.Spec.Mandatory != nil {
				return []string{shared.MrmMandatoryModuleFieldIndexPositiveValue}
			}
			return []string{shared.MrmMandatoryModuleFieldIndexNegativeValue}
		},
	)
}

// setupFieldIndexForModuleTemplateByModuleVersion sets up a field indexer on ModuleTemplates to optimize lookup by version.
// MatchingFields: "spec.version" -> "<version>"
func setupFieldIndexForModuleTemplateByModuleVersion(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1beta2.ModuleTemplate{},
		shared.ModuleTemplateVersionFieldIndexName,
		func(obj client.Object) []string {
			mt, ok := obj.(*v1beta2.ModuleTemplate)
			if !ok {
				return nil
			}
			return []string{mt.Spec.Version}
		},
	)
}

func (r *DeletionReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	// TODO: Indexed Fields setup for integration tests, where Installation Reconciler is not setup
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.ModuleReleaseMeta{}).
		Named(deletionControllerName).
		WithOptions(opts).
		WithEventFilter(predicate.And(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			mrm := obj.(*v1beta2.ModuleReleaseMeta)
			if mrm.Spec.Mandatory == nil || mrm.DeletionTimestamp.IsZero() {
				return false
			}
			return true
		}), predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Complete(reconcile.AsReconciler[*v1beta2.ModuleReleaseMeta](mgr.GetClient(), r)); err != nil {
		return fmt.Errorf("failed to setup manager for mandatory module deletion controller: %w", err)
	}
	return nil
}
