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
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
)

const (
	watcherFinalizer = "operator.kyma-project.io/watcher"
)

// WatcherReconciler reconciles a Watcher object.
type WatcherReconciler struct {
	client.Client
	*custom.IstioClient
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
	Config     *WatcherConfig
	RequeueIntervals
}

type WatcherConfig struct {
	// WebhookChartPath represents the path of the webhook chart
	// to be installed on SKR clusters upon reconciling watcher CRs.
	WebhookChartPath       string
	SkrWebhookMemoryLimits string
	SkrWebhookCPULimits    string
}

// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciliation loop start")

	watcherObj := &v1alpha1.Watcher{}
	err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcherObj)
	if err != nil {
		logger.Error(err, "Failed to get reconciliation object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !watcherObj.DeletionTimestamp.IsZero() && watcherObj.Status.State != v1alpha1.WatcherStateDeleting {
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.updateWatcherCRStatus(ctx, watcherObj, v1alpha1.WatcherStateDeleting,
			"deletion timestamp set")
	}

	// check finalizer on native object
	if !controllerutil.ContainsFinalizer(watcherObj, watcherFinalizer) {
		controllerutil.AddFinalizer(watcherObj, watcherFinalizer)
		return ctrl.Result{}, r.Update(ctx, watcherObj)
	}

	// state handling
	switch watcherObj.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, watcherObj)
	case v1alpha1.WatcherStateProcessing:
		return ctrl.Result{Requeue: true},
			r.HandleProcessingState(ctx, logger, watcherObj)
	case v1alpha1.WatcherStateDeleting:
		return ctrl.Result{Requeue: true},
			r.HandleDeletingState(ctx, logger, watcherObj)
	case v1alpha1.WatcherStateError:
		return ctrl.Result{Requeue: true},
			r.HandleProcessingState(ctx, logger, watcherObj)
	case v1alpha1.WatcherStateReady:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success},
			r.HandleProcessingState(ctx, logger, watcherObj)
	}

	return ctrl.Result{}, nil
}

func (r *WatcherReconciler) HandleInitialState(ctx context.Context, obj *v1alpha1.Watcher) error {
	return r.updateWatcherCRStatus(ctx, obj, v1alpha1.WatcherStateProcessing, "watcher cr created")
}

func (r *WatcherReconciler) HandleProcessingState(ctx context.Context,
	logger logr.Logger, obj *v1alpha1.Watcher,
) error {
	err := r.UpdateVirtualServiceConfig(ctx, obj)
	if err != nil {
		updateErr := r.updateWatcherCRStatus(ctx, obj, v1alpha1.WatcherStateError,
			"failed to create or update service mesh config")
		if updateErr == nil {
			return err
		}
		return updateErr
	}
	err = deploy.UpdateWebhookConfig(ctx, r.Config.WebhookChartPath, obj,
		r.RestConfig, r.Client, r.Config.SkrWebhookMemoryLimits, r.Config.SkrWebhookCPULimits)
	if err != nil {
		updateErr := r.updateWatcherCRStatus(ctx, obj, v1alpha1.WatcherStateError, "failed to update SKR config")
		if updateErr == nil {
			return err
		}
		return updateErr
	}
	err = r.updateWatcherCRStatus(ctx, obj, v1alpha1.WatcherStateReady, "successfully reconciled watcher cr")
	if err != nil {
		msg := "failed to update watcher cr to ready status"
		logger.Error(err, msg)
		updateErr := r.updateWatcherCRStatus(ctx, obj, v1alpha1.WatcherStateError, msg)
		if updateErr == nil {
			return err
		}
		return updateErr
	}
	logger.Info("watcher cr is Ready!")
	return nil
}

func (r *WatcherReconciler) HandleDeletingState(ctx context.Context, logger logr.Logger,
	obj *v1alpha1.Watcher,
) error {
	// remove virtual service
	err := r.RemoveVirtualServiceConfigForCR(ctx, obj)
	if err != nil {
		updateErr := r.updateWatcherCRStatus(ctx, obj, v1alpha1.WatcherStateError,
			"failed to delete service mesh config")
		if updateErr == nil {
			return err
		}
		return updateErr
	}

	// remove webhook config
	err = deploy.RemoveWebhookConfig(ctx, r.Config.WebhookChartPath, obj,
		r.RestConfig, r.Client, r.Config.SkrWebhookMemoryLimits, r.Config.SkrWebhookCPULimits)
	if err != nil {
		updateErr := r.updateWatcherCRStatus(ctx, obj, v1alpha1.WatcherStateError, "failed to delete SKR config")
		if updateErr == nil {
			return err
		}
		return updateErr
	}

	// if finalizers was removed - return
	if updated := controllerutil.RemoveFinalizer(obj, watcherFinalizer); !updated {
		return nil
	}

	if err = r.Update(ctx, obj); err != nil {
		msg := "failed to remove finalizer"
		logger.Error(err, msg)
		return err
	}
	logger.Info("deletion state handling was successful")
	return nil
}

func (r *WatcherReconciler) updateWatcherCRStatus(ctx context.Context, obj *v1alpha1.Watcher,
	state v1alpha1.WatcherState, msg string,
) error {
	obj.Status.State = state

	switch state {
	case v1alpha1.WatcherStateReady:
		obj.AddOrUpdateReadyCondition(v1alpha1.ConditionStatusTrue, msg)
	case "":
		obj.AddOrUpdateReadyCondition(v1alpha1.ConditionStatusUnknown, msg)
	case v1alpha1.WatcherStateProcessing:
		obj.AddOrUpdateReadyCondition(v1alpha1.ConditionStatusFalse, msg)
	case v1alpha1.WatcherStateDeleting:
		obj.AddOrUpdateReadyCondition(v1alpha1.ConditionStatusFalse, msg)
	case v1alpha1.WatcherStateError:
		obj.AddOrUpdateReadyCondition(v1alpha1.ConditionStatusFalse, msg)
	}

	return r.Status().Update(ctx, obj.SetObservedGeneration())
}

func (r *WatcherReconciler) SetIstioClient() error {
	//nolint:goerr113
	if r.RestConfig == nil {
		return fmt.Errorf("reconciler rest config is not set")
	}
	customIstioClient, err := custom.NewVersionedIstioClient(r.RestConfig)
	r.IstioClient = customIstioClient
	return err
}
