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

	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/pkg/util"

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
	queue.RequeueIntervals
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
	logger.V(log.DebugLevel).Info("Reconciliation loop starting")

	watcherObj := &v1beta2.Watcher{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcherObj); err != nil {
		logger.V(log.DebugLevel).Info("Failed to get reconciliation object")
		if !util.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("watcherController: %w", err)
		}
		return ctrl.Result{}, nil
	}

	if !watcherObj.DeletionTimestamp.IsZero() && watcherObj.Status.State != v1beta2.StateDeleting {
		return r.updateWatcherState(ctx, watcherObj, v1beta2.StateDeleting, nil)
	}

	// check finalizer on native object
	if !controllerutil.ContainsFinalizer(watcherObj, watcherFinalizer) {
		finalizerAdded := controllerutil.AddFinalizer(watcherObj, watcherFinalizer)
		if !finalizerAdded {
			r.EventRecorder.Event(watcherObj, "Warning", "AddFinalizerErr",
				errAddingFinalizer.Error())
			return ctrl.Result{}, errAddingFinalizer
		}
		return ctrl.Result{Requeue: true}, r.updateFinalizer(ctx, watcherObj)
	}

	watcherObj.InitializeConditions()

	return r.stateHandling(ctx, watcherObj)
}

func (r *WatcherReconciler) updateFinalizer(ctx context.Context, watcherCR *v1beta2.Watcher) error {
	err := r.Client.Update(ctx, watcherCR)
	if err != nil {
		r.EventRecorder.Event(watcherCR, "Warning", "WatcherFinalizerErr",
			err.Error())
		return fmt.Errorf("failed to update finalizer: %w", err)
	}
	return nil
}

func (r *WatcherReconciler) stateHandling(ctx context.Context, watcherCR *v1beta2.Watcher) (ctrl.Result, error) {
	switch watcherCR.Status.State {
	case "":
		return r.updateWatcherState(ctx, watcherCR, v1beta2.StateProcessing, nil)
	case v1beta2.StateProcessing:
		return r.handleProcessingState(ctx, watcherCR)
	case v1beta2.StateDeleting:
		return r.handleDeletingState(ctx, watcherCR)
	case v1beta2.StateError:
		return r.handleProcessingState(ctx, watcherCR)
	case v1beta2.StateReady, v1beta2.StateWarning:
		return r.handleProcessingState(ctx, watcherCR)
	}

	return ctrl.Result{}, nil
}

func (r *WatcherReconciler) handleDeletingState(ctx context.Context, watcherCR *v1beta2.Watcher) (ctrl.Result, error) {
	err := r.IstioClient.RemoveVirtualServiceForCR(ctx, client.ObjectKeyFromObject(watcherCR))
	if err != nil {
		vsConfigDelErr := fmt.Errorf("failed to delete virtual service (config): %w", err)
		return r.updateWatcherState(ctx, watcherCR, v1beta2.StateError, vsConfigDelErr)
	}
	finalizerRemoved := controllerutil.RemoveFinalizer(watcherCR, watcherFinalizer)
	if !finalizerRemoved {
		return r.updateWatcherState(ctx, watcherCR, v1beta2.StateError, errRemovingFinalizer)
	}
	return ctrl.Result{Requeue: true}, r.updateFinalizer(ctx, watcherCR)
}

func (r *WatcherReconciler) handleProcessingState(ctx context.Context,
	watcherCR *v1beta2.Watcher,
) (ctrl.Result, error) {
	// Create virtualService in Memory
	virtualSvc, err := r.IstioClient.NewVirtualService(ctx, watcherCR)
	if err != nil {
		return r.updateWatcherState(ctx, watcherCR, v1beta2.StateError, err)
	}

	virtualSvcRemote, err := r.IstioClient.GetVirtualService(ctx, watcherCR.Name)
	if client.IgnoreNotFound(err) != nil {
		return r.updateWatcherState(ctx, watcherCR, v1beta2.StateError, err)
	}
	if util.IsNotFound(err) {
		err = r.IstioClient.CreateVirtualService(ctx, virtualSvc)
		if err != nil {
			vsCreateErr := fmt.Errorf("failed to create virtual service: %w", err)
			return r.updateWatcherState(ctx, watcherCR, v1beta2.StateError, vsCreateErr)
		}
		return r.updateWatcherState(ctx, watcherCR, v1beta2.StateReady, nil)
	}

	err = r.IstioClient.UpdateVirtualService(ctx, virtualSvc, virtualSvcRemote)
	if err != nil {
		vsUpdateErr := fmt.Errorf("failed to update virtual service: %w", err)
		return r.updateWatcherState(ctx, watcherCR, v1beta2.StateError, vsUpdateErr)
	}
	return r.updateWatcherState(ctx, watcherCR, v1beta2.StateReady, nil)
}

func (r *WatcherReconciler) updateWatcherState(ctx context.Context, watcherCR *v1beta2.Watcher,
	state v1beta2.State, err error,
) (ctrl.Result, error) {
	watcherCR.Status.State = state
	if state == v1beta2.StateReady {
		watcherCR.UpdateWatcherConditionStatus(v1beta2.WatcherConditionTypeVirtualService, metav1.ConditionTrue)
	} else if state == v1beta2.StateError {
		watcherCR.UpdateWatcherConditionStatus(v1beta2.WatcherConditionTypeVirtualService, metav1.ConditionFalse)
	}
	if err != nil {
		r.EventRecorder.Event(watcherCR, "Warning", "WatcherStatusUpdate", err.Error())
	}
	requeueInterval := queue.DetermineRequeueInterval(state, r.RequeueIntervals)
	return ctrl.Result{RequeueAfter: requeueInterval}, r.updateWatcherStatusUsingSSA(ctx, watcherCR)
}

func (r *WatcherReconciler) updateWatcherStatusUsingSSA(ctx context.Context, watcher *v1beta2.Watcher) error {
	watcher.ManagedFields = nil
	err := r.Client.Status().Patch(ctx, watcher, client.Apply, client.FieldOwner(v1beta2.OperatorName),
		status.SubResourceOpts(client.ForceOwnership))
	if err != nil {
		reason := "WatcherStatusUpdate"
		r.EventRecorder.Event(watcher, "Warning", reason, err.Error())
		return fmt.Errorf("%s failed: %w", reason, err)
	}
	return nil
}
