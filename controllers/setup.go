package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"


	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/pkg/index"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/security"
	"github.com/kyma-project/lifecycle-manager/pkg/watch"
	listener "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

type SetupUpSetting struct {
	ListenerAddr                 string
	EnableDomainNameVerification bool
	IstioNamespace               string
}

const (
	WatcherControllerName = "watcher"
)

var (
	errConvertingWatched      = errors.New("error converting watched to object key")
	errParsingWatched         = errors.New("error parsing watched from watcher event")
	errConvertingWatcherEvent = errors.New("error converting watcher event to unstructured")
)

// SetupWithManager sets up the Kyma controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager,
	options controller.Options, settings SetupUpSetting,
) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&v1beta1.Kyma{}).WithOptions(options).
		Watches(
			&source.Kind{Type: &v1beta1.ModuleTemplate{}},
			handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch(context.TODO())),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		// here we define a watch on secrets for the lifecycle-manager so that the cache is picking up changes
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.Funcs{})

	controllerBuilder = controllerBuilder.Watches(&source.Kind{Type: &v1beta1.Manifest{}},
		&watch.RestrictedEnqueueRequestForOwner{Log: ctrl.Log, OwnerType: &v1beta1.Kyma{}, IsController: true})

	var runnableListener *listener.SKREventListener
	var eventChannel *source.Channel
	var verifyFunc listener.Verify

	if settings.EnableDomainNameVerification {
		// Verifier used to verify incoming listener requests
		verifyFunc = security.NewRequestVerifier(mgr.GetClient()).Verify
	} else {
		verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		}
	}
	// register listener component incl. domain name verification
	runnableListener, eventChannel = listener.RegisterListenerComponent(
		settings.ListenerAddr,
		v1beta1.OperatorName,
		verifyFunc,
	)

	// watch event channel
	r.watchEventChannel(controllerBuilder, eventChannel)
	// start listener as a manager runnable
	if err := mgr.Add(runnableListener); err != nil {
		return err
	}

	if err := r.configureIndexing(context.TODO(), mgr); err != nil {
		return err
	}

	if err := controllerBuilder.Complete(r); err != nil {
		return fmt.Errorf("error occurred while building controller: %w", err)
	}

	return nil
}

func (r *KymaReconciler) configureIndexing(ctx context.Context, mgr ctrl.Manager) error {
	if err := index.TemplateChannel().With(ctx, mgr.GetFieldIndexer()); err != nil {
		return fmt.Errorf(
			"error while setting up ModuleTemplate Channel Field Indexer, "+
				"make sure you installed all CRDs: %w", err,
		)
	}
	if err := index.TemplateFQDN().With(ctx, mgr.GetFieldIndexer()); err != nil {
		return fmt.Errorf(
			"error while setting up ModuleTemplate FQDN Field Indexer, "+
				"make sure you installed all CRDs: %w", err,
		)
	}
	if err := index.TemplateName().With(ctx, mgr.GetFieldIndexer()); err != nil {
		return fmt.Errorf(
			"error while setting up ModuleTemplate Name Field Indexer, "+
				"make sure you installed all CRDs: %w", err,
		)
	}
	return nil
}

func (r *KymaReconciler) watchEventChannel(controllerBuilder *builder.Builder, eventChannel *source.Channel) {
	controllerBuilder.Watches(eventChannel, &handler.Funcs{
		GenericFunc: func(event event.GenericEvent, queue workqueue.RateLimitingInterface) {
			logger := ctrl.Log.WithName("listener")
			unstructWatcherEvt, conversionOk := event.Object.(*unstructured.Unstructured)
			if !conversionOk {
				logger.Error(errConvertingWatcherEvent, fmt.Sprintf("event: %v", event.Object))
				return
			}
			watched, ok := unstructWatcherEvt.Object["watched"]
			if !ok {
				logger.Error(errParsingWatched, fmt.Sprintf("unstructured event: %v", unstructWatcherEvt))
				return
			}
			watchedObjectKey, conversionOk := watched.(client.ObjectKey)
			if !conversionOk {
				logger.Error(errConvertingWatched, fmt.Sprintf("watched object: %v", watched))
				return
			}
			logger.Info(
				fmt.Sprintf("event coming from SKR, adding %s to queue",
					watchedObjectKey.String()),
			)

			queue.Add(ctrl.Request{
				NamespacedName: watchedObjectKey,
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
		For(&v1beta1.Watcher{}).
		Named(WatcherControllerName).
		WithOptions(options).
		Complete(r)
}
