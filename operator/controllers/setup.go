package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/dynamic"
	"github.com/kyma-project/kyma-operator/operator/pkg/index"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	"github.com/kyma-project/kyma-watcher/listener"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(
	mgr ctrl.Manager,
	options controller.Options,
	listenerAddr string,
) error {

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Kyma{}).WithOptions(options).
		Watches(
			&source.Kind{Type: &v1alpha1.ModuleTemplate{}},
			handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch(context.TODO())),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		// here we define a watch on secrets for the kyma operator so that the cache is picking up changes
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.Funcs{})

	// This fetches all resources for our component operator CRDs, might become a problem if component operators
	// create their own CRDs that we dont need to watch
	dynamicInformers, err := dynamic.Informers(mgr, []string{v1alpha1.ComponentPrefix})
	if err != nil {
		return fmt.Errorf("error while setting up Dynamic Informers: %w", err)
	}

	for _, informer := range dynamicInformers {
		controllerBuilder = controllerBuilder.
			Watches(informer, &handler.Funcs{UpdateFunc: watch.NewComponentChangeHandler(r).Watch(context.TODO())},
				builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}))
	}

	// register listener component
	runnableListener, eventChannel := listener.RegisterListenerComponent(
		listenerAddr, strings.ToLower(v1alpha1.KymaKind))
	// watch event channel
	controllerBuilder.Watches(eventChannel, &handler.EnqueueRequestForObject{})
	// start listener as a manager runnable
	if err = mgr.Add(runnableListener); err != nil {
		return err
	}

	if err := index.TemplateChannel().With(context.TODO(), mgr.GetFieldIndexer()); err != nil {
		return fmt.Errorf("error while setting up ModuleTemplate Channel Field Indexer, "+
			"make sure you installed all CRDs: %w", err)
	}

	if err := controllerBuilder.Complete(r); err != nil {
		return fmt.Errorf("error occurred while building controller: %w", err)
	}

	return nil
}
