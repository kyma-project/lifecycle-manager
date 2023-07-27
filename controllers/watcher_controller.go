/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	watcherFinalizer = "operator.kyma-project.io/watcher"
)

var (
	errRestConfigIsNotSet = errors.New("reconciler rest config is not set")
	errRemovingFinalizer  = errors.New("error removing finalizer")
	errAddingFinalizer    = errors.New("error adding finalizer")
)

// WatcherReconciler reconciles a Watcher object.
type WatcherReconciler struct {
	client.Client
	record.EventRecorder
	IstioClient *istio.Client
	RestConfig  *rest.Config
	Scheme      *runtime.Scheme
	RequeueIntervals
}

// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrlLog.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("Reconciliation loop starting")

	watcherObj := &v1beta2.Watcher{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcherObj); err != nil {
		logger.V(log.DebugLevel).Info("Failed to get reconciliation object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !watcherObj.DeletionTimestamp.IsZero() && watcherObj.Status.State != v1beta2.WatcherStateDeleting {
		return ctrl.Result{}, r.updateWatcherState(ctx, watcherObj, v1beta2.WatcherStateDeleting)
	}

	// check finalizer on native object
	if !controllerutil.ContainsFinalizer(watcherObj, watcherFinalizer) {
		finalizerAdded := controllerutil.AddFinalizer(watcherObj, watcherFinalizer)
		if !finalizerAdded {
			r.EventRecorder.Event(watcherObj, "Warning", "AddFinalizerErr",
				errAddingFinalizer.Error())
			return ctrl.Result{}, errAddingFinalizer
		}
		return ctrl.Result{}, r.updateFinalizer(ctx, watcherObj)
	}

	watcherObj.InitializeConditions()

	return r.stateHandling(ctx, watcherObj)
}

func (r *WatcherReconciler) updateFinalizer(ctx context.Context, watcherCR *v1beta2.Watcher) error {
	err := r.Client.Update(ctx, watcherCR)
	if err != nil {
		r.EventRecorder.Event(watcherCR, "Warning", "WatcherFinalizerErr",
			err.Error())
		return err
	}
	return nil
}

func (r *WatcherReconciler) stateHandling(ctx context.Context, watcherCR *v1beta2.Watcher) (ctrl.Result, error) {
	switch watcherCR.Status.State {
	case "":
		return ctrl.Result{}, r.updateWatcherState(ctx, watcherCR, v1beta2.WatcherStateProcessing)
	case v1beta2.WatcherStateProcessing:
		return ctrl.Result{Requeue: true}, r.handleProcessingState(ctx, watcherCR)
	case v1beta2.WatcherStateDeleting:
		return ctrl.Result{}, r.handleDeletingState(ctx, watcherCR)
	case v1beta2.WatcherStateError:
		return ctrl.Result{Requeue: true}, r.handleProcessingState(ctx, watcherCR)
	case v1beta2.WatcherStateReady:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, r.handleProcessingState(ctx, watcherCR)
	}

	return ctrl.Result{}, nil
}

func (r *WatcherReconciler) handleDeletingState(ctx context.Context, watcherCR *v1beta2.Watcher) error {
	err := r.IstioClient.RemoveVirtualServiceForCR(ctx, client.ObjectKeyFromObject(watcherCR))
	if err != nil {
		vsConfigDelErr := fmt.Errorf("failed to delete virtual service (config): %w", err)
		r.EventRecorder.Event(watcherCR, "Warning", "WatcherDeletionErr", err.Error())
		return vsConfigDelErr
	}
	finalizerRemoved := controllerutil.RemoveFinalizer(watcherCR, watcherFinalizer)
	if !finalizerRemoved {
		r.EventRecorder.Event(watcherCR, "Warning", "RemoveFinalizerErr",
			errRemovingFinalizer.Error())
		return errRemovingFinalizer
	}
	return r.updateFinalizer(ctx, watcherCR)
}

func (r *WatcherReconciler) handleProcessingState(ctx context.Context, watcherCR *v1beta2.Watcher) error {
	// Create virtualService in Memory
	virtualSvc, err := r.IstioClient.NewVirtualService(ctx, watcherCR)
	if err != nil {
		return err
	}

	virtualSvcRemote, err := r.IstioClient.GetVirtualService(ctx, watcherCR.Name)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if util.IsNotFound(err) {
		err = r.IstioClient.CreateVirtualService(ctx, virtualSvc)
		if err != nil {
			vsCreateErr := fmt.Errorf("failed to create virtual service: %w", err)
			return r.updateWatcherToErrState(ctx, watcherCR, vsCreateErr)
		}
		return r.updateWatcherState(ctx, watcherCR, v1beta2.WatcherStateReady)
	}

	err = r.IstioClient.UpdateVirtualService(ctx, virtualSvc, virtualSvcRemote)
	if err != nil {
		vsUpdateErr := fmt.Errorf("failed to update virtual service: %w", err)
		return r.updateWatcherToErrState(ctx, watcherCR, vsUpdateErr)
	}
	return r.updateWatcherState(ctx, watcherCR, v1beta2.WatcherStateReady)
}

func (r *WatcherReconciler) updateWatcherState(ctx context.Context, watcherCR *v1beta2.Watcher,
	state v1beta2.WatcherState,
) error {
	watcherCR.Status.State = state
	if state == v1beta2.WatcherStateReady {
		watcherCR.UpdateWatcherConditionStatus(v1beta2.WatcherConditionTypeVirtualService, metav1.ConditionTrue)
	}
	return r.updateWatcherStatusUsingSSA(ctx, watcherCR)
}

func (r *WatcherReconciler) updateWatcherToErrState(ctx context.Context, watcherCR *v1beta2.Watcher, err error) error {
	watcherCR.Status.State = v1beta2.WatcherStateError
	watcherCR.UpdateWatcherConditionStatus(v1beta2.WatcherConditionTypeVirtualService, metav1.ConditionFalse)
	r.EventRecorder.Event(watcherCR, "Warning", "WatcherStatusUpdate", err.Error())
	// always return non nil err to requeue the CR for another reconciliation.
	updateErr := r.updateWatcherStatusUsingSSA(ctx, watcherCR)
	if updateErr != nil {
		return updateErr
	}
	return err
}

func (r *WatcherReconciler) updateWatcherStatusUsingSSA(ctx context.Context, watcher *v1beta2.Watcher) error {
	watcher.ManagedFields = nil
	reason := "WatcherStatusUpdate"
	err := r.Client.Status().Patch(ctx, watcher, client.Apply, client.FieldOwner(v1beta2.OperatorName),
		status.SubResourceOpts(client.ForceOwnership))
	if err != nil {
		r.EventRecorder.Event(watcher, "Warning", reason, err.Error())
		return fmt.Errorf("%s failed: %w", reason, err)
	}
	return nil
}
