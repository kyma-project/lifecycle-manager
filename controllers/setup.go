package controllers

import (
	"context"
	"fmt"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/kyma-project/lifecycle-manager/pkg/istio"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/index"
	"github.com/kyma-project/lifecycle-manager/pkg/watch"
	moduleManagerV1alpha1 "github.com/kyma-project/module-manager/api/v1alpha1"
	listener "github.com/kyma-project/runtime-watcher/listener/pkg/event"
)

const (
	WatcherControllerName = "watcher"
)

// SetupWithManager sets up the Kyma controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options, listenerAddr string) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Kyma{}).WithOptions(options).
		Watches(
			&source.Kind{Type: &v1alpha1.ModuleTemplate{}},
			handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch(context.TODO())),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		// here we define a watch on secrets for the lifecycle-manager so that the cache is picking up changes
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.Funcs{})

	controllerBuilder = controllerBuilder.Watches(&source.Kind{Type: &moduleManagerV1alpha1.Manifest{}},
		&watch.RestrictedEnqueueRequestForOwner{
			Log: ctrl.Log, OwnerType: &v1alpha1.Kyma{}, IsController: true,
		})

	// register listener component
	runnableListener, eventChannel := listener.RegisterListenerComponent(
		listenerAddr, v1alpha1.OperatorName,
	)

	// watch event channel
	r.watchEventChannel(controllerBuilder, eventChannel)
	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
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

func (r *KymaReconciler) watchEventChannel(controllerBuilder *builder.Builder, eventChannel *source.Channel) {
	controllerBuilder.Watches(eventChannel, &handler.Funcs{
		GenericFunc: func(event event.GenericEvent, queue workqueue.RateLimitingInterface) {
			ctrl.Log.WithName("listener").Info(
				fmt.Sprintf("event coming from SKR, adding %s to queue",
					client.ObjectKeyFromObject(event.Object).String()),
			)

			queue.Add(ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(event.Object),
			})
		},
	})
}

// SetupWithManager sets up the Watcher controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options,
	istioConfig istio.Config,
) error {
	if r.RestConfig == nil {
		return ErrRestConfigIsNotSet
	}
	var err error
	r.IstioClient, err = istio.NewVersionedIstioClient(r.RestConfig, istioConfig, r.EventRecorder,
		ctrl.Log.WithName("istioClient"))
	if err != nil {
		return fmt.Errorf("unable to set istio client for watcher controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Watcher{}).
		Named(WatcherControllerName).
		WithOptions(options).
		Complete(r)
}
