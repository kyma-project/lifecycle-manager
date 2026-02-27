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

package watcher

import (
	"context"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	addFinalizerFailure        event.Reason = "AddFinalizerErr"
	updateFinalizerFailure     event.Reason = "WatcherFinalizerErr"
	gatewayNotFoundFailure     event.Reason = "WatcherGatewayNotFound"
	watcherStatusUpdateFailure event.Reason = "WatcherStatusUpdate"
)

var (
	errFinalizerRemove = errors.New("error removing finalizer")
	errFinalizerAdd    = errors.New("error adding finalizer")
	errGateway         = errors.New("gateway for the VirtualService not found")
)

type Reconciler struct {
	client.Client
	event.Event
	queue.RequeueIntervals

	RateLimiter workqueue.TypedRateLimiter[ctrl.Request]

	IstioClient           *istio.Client
	VirtualServiceFactory istio.VirtualServiceFactory
	RestConfig            *rest.Config
	Scheme                *machineryruntime.Scheme
	IstioGatewayNamespace string
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).WithName(req.String())
	logger.V(log.DebugLevel).Info("Reconciliation loop starting")

	watcher := &v1beta2.Watcher{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcher); err != nil {
		logger.V(log.DebugLevel).Info("Failed to get reconciliation object")
		if !util.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("watcherController: %w", err)
		}
		return ctrl.Result{}, nil
	}

	if !watcher.DeletionTimestamp.IsZero() && watcher.Status.State != shared.StateDeleting {
		return r.updateWatcherState(ctx, watcher, shared.StateDeleting, nil)
	}

	if !controllerutil.ContainsFinalizer(watcher, shared.WatcherFinalizer) {
		if !controllerutil.AddFinalizer(watcher, shared.WatcherFinalizer) {
			r.Event.Warning(watcher, addFinalizerFailure, errFinalizerAdd)
			return ctrl.Result{}, errFinalizerAdd
		}
		if err := r.updateFinalizer(ctx, watcher); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
	}

	watcher.InitializeConditions()

	return r.stateHandling(ctx, req, watcher)
}

func (r *Reconciler) updateFinalizer(ctx context.Context, watcher *v1beta2.Watcher) error {
	err := r.Update(ctx, watcher)
	if err != nil {
		r.Event.Warning(watcher, updateFinalizerFailure, err)
		return fmt.Errorf("failed to update finalizer: %w", err)
	}
	return nil
}

func (r *Reconciler) stateHandling(
	ctx context.Context, req ctrl.Request, watcher *v1beta2.Watcher,
) (ctrl.Result, error) {
	switch watcher.Status.State {
	case "":
		return r.updateWatcherState(ctx, watcher, shared.StateProcessing, nil)
	case shared.StateProcessing:
		return r.handleProcessingState(ctx, watcher)
	case shared.StateDeleting:
		return r.handleDeletingState(ctx, req, watcher)
	case shared.StateError:
		return r.handleProcessingState(ctx, watcher)
	case shared.StateReady, shared.StateWarning:
		return r.handleProcessingState(ctx, watcher)
	case shared.StateUnmanaged:
		return ctrl.Result{}, nil // no requeue of invalid state
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) handleDeletingState(
	ctx context.Context, req ctrl.Request, watcher *v1beta2.Watcher,
) (ctrl.Result, error) {
	err := r.IstioClient.DeleteVirtualService(ctx, watcher.GetName(), watcher.GetNamespace())
	if err != nil && !util.IsNotFound(err) {
		vsConfigDelErr := fmt.Errorf("failed to delete virtual service (config): %w", err)
		return r.updateWatcherState(ctx, watcher, shared.StateError, vsConfigDelErr)
	}
	finalizerRemoved := controllerutil.RemoveFinalizer(watcher, shared.WatcherFinalizer)
	if !finalizerRemoved {
		return r.updateWatcherState(ctx, watcher, shared.StateError, errFinalizerRemove)
	}
	if err := r.updateFinalizer(ctx, watcher); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
}

func (r *Reconciler) handleProcessingState(ctx context.Context, watcherCR *v1beta2.Watcher) (ctrl.Result, error) {
	gateways, err := r.IstioClient.ListGatewaysByLabelSelector(ctx, &watcherCR.Spec.Gateway.LabelSelector,
		r.IstioGatewayNamespace)
	if err != nil || len(gateways.Items) == 0 {
		r.Event.Warning(watcherCR, gatewayNotFoundFailure, errGateway)
		return r.updateWatcherState(ctx, watcherCR, shared.StateError, err)
	}

	virtualSvc, err := r.VirtualServiceFactory.NewVirtualService(watcherCR, gateways)
	if err != nil {
		return r.updateWatcherState(ctx, watcherCR, shared.StateError, err)
	}

	virtualSvcRemote, err := r.IstioClient.GetVirtualService(ctx, watcherCR.GetName(), watcherCR.GetNamespace())
	if client.IgnoreNotFound(err) != nil {
		return r.updateWatcherState(ctx, watcherCR, shared.StateError, err)
	}
	if util.IsNotFound(err) {
		err = r.IstioClient.CreateVirtualService(ctx, virtualSvc)
		if err != nil {
			vsCreateErr := fmt.Errorf("failed to create virtual service: %w", err)
			return r.updateWatcherState(ctx, watcherCR, shared.StateError, vsCreateErr)
		}
		return r.updateWatcherState(ctx, watcherCR, shared.StateReady, nil)
	}

	err = r.IstioClient.UpdateVirtualService(ctx, virtualSvc, virtualSvcRemote)
	if err != nil {
		vsUpdateErr := fmt.Errorf("failed to update virtual service: %w", err)
		return r.updateWatcherState(ctx, watcherCR, shared.StateError, vsUpdateErr)
	}
	return r.updateWatcherState(ctx, watcherCR, shared.StateReady, nil)
}

func (r *Reconciler) updateWatcherState(ctx context.Context, watcher *v1beta2.Watcher, state shared.State,
	err error,
) (ctrl.Result, error) {
	watcher.Status.State = state
	switch state {
	case shared.StateReady:
		watcher.UpdateWatcherConditionStatus(v1beta2.WatcherConditionTypeVirtualService, apimetav1.ConditionTrue)
	case shared.StateError:
		watcher.UpdateWatcherConditionStatus(v1beta2.WatcherConditionTypeVirtualService, apimetav1.ConditionFalse)
	case shared.StateWarning:
	case shared.StateProcessing:
	case shared.StateDeleting:
	case shared.StateUnmanaged:
	}

	if err != nil {
		r.Event.Warning(watcher, watcherStatusUpdateFailure, err)
	}
	requeueInterval := queue.DetermineRequeueInterval(state, r.RequeueIntervals)
	return ctrl.Result{RequeueAfter: requeueInterval}, r.updateWatcherStatusUsingSSA(ctx, watcher)
}

func (r *Reconciler) updateWatcherStatusUsingSSA(ctx context.Context, watcher *v1beta2.Watcher) error {
	watcher.ManagedFields = nil
	err := r.Client.Status().Patch(ctx, watcher, client.Apply, fieldowners.LegacyLifecycleManager,
		status.SubResourceOpts(client.ForceOwnership))
	if err != nil {
		err = fmt.Errorf("watcher status update failed: %w", err)
		r.Event.Warning(watcher, watcherStatusUpdateFailure, err)
		return err
	}
	return nil
}
