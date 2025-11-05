package mandatorymodule

import (
	"context"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldindex"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
)

const (
	installationControllerName = "mandatory-module-installation"
	deletionControllerName     = "mandatory-module-deletion"
)

func (r *InstallationReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	err := setupFieldIndexForMandatoryMrm(mgr)
	if err != nil {
		return err
	}

	err = setupFieldIndexForModuleTemplateByModuleVersion(mgr)
	if err != nil {
		return err
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
// MatchingFields: "is-mandatory" -> "true"/"false".
func setupFieldIndexForMandatoryMrm(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1beta2.ModuleReleaseMeta{},
		fieldindex.MrmMandatoryModuleName,
		func(obj client.Object) []string {
			mrm, ok := obj.(*v1beta2.ModuleReleaseMeta)
			if !ok {
				return nil
			}
			if mrm.Spec.Mandatory != nil {
				return []string{fieldindex.MrmMandatoryModulePositiveValue}
			}
			return []string{fieldindex.MrmMandatoryModuleNegativeValue}
		},
	)
	if err != nil {
		return fmt.Errorf("failed to index field for mandatory MRMs: %w", err)
	}
	return nil
}

// setupFieldIndexForModuleTemplateByModuleVersion sets up a field indexer on ModuleTemplates to optimize lookup by version.
// MatchingFields: "spec.version" -> "<version>".
func setupFieldIndexForModuleTemplateByModuleVersion(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1beta2.ModuleTemplate{},
		fieldindex.ModuleTemplateVersionName,
		func(obj client.Object) []string {
			mt, ok := obj.(*v1beta2.ModuleTemplate)
			if !ok {
				return nil
			}
			return []string{mt.Spec.Version}
		},
	)
	if err != nil {
		return fmt.Errorf("failed to index field for ModuleTemplate by version: %w", err)
	}
	return nil
}

func (r *DeletionReconciler) SetupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.ModuleReleaseMeta{}).
		Named(deletionControllerName).
		WithOptions(opts).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			mrm, ok := obj.(*v1beta2.ModuleReleaseMeta)
			if !ok {
				return false
			}
			return mrm.Spec.Mandatory != nil
		})).
		Complete(reconcile.AsReconciler[*v1beta2.ModuleReleaseMeta](mgr.GetClient(), r)); err != nil {
		return fmt.Errorf("failed to setup manager for mandatory module deletion controller: %w", err)
	}
	return nil
}
