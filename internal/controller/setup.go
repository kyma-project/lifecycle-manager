package controller

import (
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/watch"
)

type SetupUpSetting struct {
	ListenerAddr                 string
	EnableDomainNameVerification bool
	IstioNamespace               string
}

const (
	WatcherControllerName                     = "watcher"
	PurgeControllerName                       = "purge"
	ManifestControllerName                    = "manifest"
	MandatoryModuleInstallationControllerName = "mandatory-module-installation"
	MandatoryModuleDeletionControllerName     = "mandatory-module-deletion"
)

<<<<<<< HEAD
=======
var (
	errConvertingWatched      = errors.New("error converting watched to object key")
	errParsingWatched         = errors.New("error getting watched object from unstructured event")
	errConvertingWatcherEvent = errors.New("error converting watched object to unstructured event")
)

// SetupWithManager sets up the Kyma controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager,
	options ctrlruntime.Options, settings SetupUpSetting,
) error {
	predicates := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&v1beta2.Kyma{}).
		Named(KymaControllerName).
		WithOptions(options).
		WithEventFilter(predicates).
		Watches(
			&v1beta2.ModuleTemplate{},
			handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch()),
			builder.WithPredicates(predicates),
		).
		// here we define a watch on secrets for the lifecycle-manager so that the cache is picking up changes
		Watches(&apicorev1.Secret{}, handler.Funcs{})

	controllerBuilder = controllerBuilder.Watches(&v1beta2.Manifest{},
		&watch.RestrictedEnqueueRequestForOwner{Log: ctrl.Log, OwnerType: &v1beta2.Kyma{}, IsController: true})

	var runnableListener *watcherevent.SKREventListener
	var verifyFunc watcherevent.Verify

	if settings.EnableDomainNameVerification {
		// Verifier used to verify incoming listener requests
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}
	// register listener component incl. domain name verification
	runnableListener = watcherevent.NewSKREventListener(
		settings.ListenerAddr,
		shared.OperatorName,
		verifyFunc,
	)

	// watch event channel
	controllerBuilder.WatchesRawSource(source.Channel(runnableListener.ReceivedEvents, r.skrEventHandler()))

	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
		return fmt.Errorf("KymaReconciler %w", err)
	}

	if err := controllerBuilder.Complete(r); err != nil {
		return fmt.Errorf("error occurred while building controller: %w", err)
	}

	return nil
}

func (r *KymaReconciler) skrEventHandler() *handler.Funcs {
	return &handler.Funcs{
		GenericFunc: func(ctx context.Context, evnt event.GenericEvent, queue workqueue.RateLimitingInterface) {
			logger := ctrl.Log.WithName("listener")
			unstructWatcherEvt, conversionOk := evnt.Object.(*unstructured.Unstructured)
			if !conversionOk {
				logger.Error(errConvertingWatcherEvent, fmt.Sprintf("event: %v", evnt.Object))
				return
			}

			// get owner object from unstructured event, owner = KymaCR object reference in KCP
			unstructuredOwner, ok := unstructWatcherEvt.Object["owner"]
			if !ok {
				logger.Error(errParsingWatched, fmt.Sprintf("unstructured event: %v", unstructWatcherEvt))
				return
			}

			ownerObjectKey, conversionOk := unstructuredOwner.(client.ObjectKey)
			if !conversionOk {
				logger.Error(errConvertingWatched, fmt.Sprintf("unstructured Owner object: %v", unstructuredOwner))
				return
			}

			logger.Info(
				fmt.Sprintf("event received from SKR, adding %s to queue",
					ownerObjectKey),
			)

			queue.Add(ctrl.Request{
				NamespacedName: ownerObjectKey,
			})
		},
	}
}

>>>>>>> upstream/main
// SetupWithManager sets up the Watcher controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager, options ctrlruntime.Options,
) error {
	if r.RestConfig == nil {
		return errRestConfigIsNotSet
	}
	var err error
	r.IstioClient, err = istio.NewIstioClient(r.RestConfig, r.EventRecorder,
		ctrl.Log.WithName("istioClient"))
	if err != nil {
		return fmt.Errorf("unable to set istio client for watcher controller: %w", err)
	}

	r.VirtualServiceFactory, err = istio.NewVirtualServiceService(r.Scheme)
	if err != nil {
		return fmt.Errorf("unable to set VirtualService service for watcher controller: %w", err)
	}

	ctrlManager := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Watcher{}).
		Named(WatcherControllerName).
		WithOptions(options).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))

	err = ctrlManager.Complete(r)
	if err != nil {
		return fmt.Errorf("failed to setup manager for watcher controller: %w", err)
	}
	return nil
}

// SetupWithManager sets up the Purge controller with the Manager.
func (r *PurgeReconciler) SetupWithManager(mgr ctrl.Manager,
	options ctrlruntime.Options,
) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Kyma{}).
		Named(PurgeControllerName).
		WithOptions(options).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))

	if err := controllerBuilder.Complete(r); err != nil {
		return fmt.Errorf("error occurred while building controller: %w", err)
	}

	return nil
}

// SetupWithManager sets up the MandatoryModuleReconciler with the Manager.
func (r *MandatoryModuleReconciler) SetupWithManager(mgr ctrl.Manager,
	options ctrlruntime.Options,
) error {
	predicates := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Kyma{}).
		Named(MandatoryModuleInstallationControllerName).
		WithOptions(options).
		WithEventFilter(predicates).
		Watches(
			&v1beta2.ModuleTemplate{},
			handler.EnqueueRequestsFromMapFunc(watch.NewMandatoryTemplateChangeHandler(r).Watch()),
			builder.WithPredicates(predicates),
		).
		Watches(&apicorev1.Secret{}, handler.Funcs{})

	if err := controllerBuilder.Complete(r); err != nil {
		return fmt.Errorf("error occurred while building controller: %w", err)
	}

	return nil
}

// SetupWithManager sets up the MandatoryModuleDeletionReconciler with the Manager.
func (r *MandatoryModuleDeletionReconciler) SetupWithManager(mgr ctrl.Manager,
	options ctrlruntime.Options,
) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.ModuleTemplate{}).
		Named(MandatoryModuleDeletionControllerName).
		WithOptions(options).
		WithEventFilter(predicate.GenerationChangedPredicate{})

	if err := controllerBuilder.Complete(r); err != nil {
		return fmt.Errorf("error occurred while building controller: %w", err)
	}

	return nil
}
