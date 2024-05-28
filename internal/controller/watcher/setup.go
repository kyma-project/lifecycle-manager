package watcher

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
)

const controllerName = "watcher"

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, options ctrlruntime.Options) error {
	if r.RestConfig == nil {
		return errRestConfigIsNotSet
	}
	var err error
	r.IstioClient, err = istio.NewIstioClient(r.RestConfig, r.EventRecorder, ctrl.Log.WithName("istioClient"))
	if err != nil {
		return fmt.Errorf("unable to set istio client for watcher controller: %w", err)
	}

	r.VirtualServiceFactory, err = istio.NewVirtualServiceService(r.Scheme)
	if err != nil {
		return fmt.Errorf("unable to set VirtualService service for watcher controller: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Watcher{}).
		Named(controllerName).
		WithOptions(options).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})).
		Complete(r)
}
