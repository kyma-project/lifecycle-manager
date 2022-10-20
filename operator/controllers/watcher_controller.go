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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
)

// WatcherReconciler reconciles a Watcher object.
type WatcherReconciler struct {
	client.Client
	*custom.IstioClient
	*deploy.SKRChartManager
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
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String()).WithValues("reconciler", "watcher")
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	// check if kyma resource exists
	kyma := &v1alpha1.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		if apierrors.IsNotFound(err) {
			logger.Info(req.NamespacedName.String() + " got deleted!")
			err := r.RemoveWebhookChart(ctx, &v1alpha1.WatcherList{Items: []v1alpha1.Watcher{}}, req.NamespacedName, r.RestConfig)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err //nolint:wrapcheck
	}

	// check if deletionTimestamp is set, remove webhook chart on that SKR
	//if !kyma.DeletionTimestamp.IsZero() {
	//	err := r.RemoveWebhookChart(ctx, &v1alpha1.WatcherList{Items: []v1alpha1.Watcher{}}, req.NamespacedName, r.RestConfig)
	//	return ctrl.Result{}, err //nolint:wrapcheck
	//}

	if err := r.ConfigureWatchersForSKR(ctx, logger, kyma); err != nil {
		logger.Error(err, "error configuring watchers for skr")
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
	}
	logger.Info("Configured watchers for SKR successfully!")
	return ctrl.Result{}, nil
}

func (r *WatcherReconciler) ConfigureWatchersForSKR(ctx context.Context,
	logger logr.Logger, kyma *v1alpha1.Kyma,
) error {

	watcherList := &v1alpha1.WatcherList{}
	if err := r.List(ctx, watcherList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("error listing watcher resources: %w", err)
	}

	if err := r.ConfigureVirtualService(ctx, watcherList.Items); err != nil {
		updateErr := r.UpdateWatchersStatusesToError(ctx, logger, watcherList, "error configuring istio resources")
		if updateErr == nil {
			return err
		}
		return updateErr
	}

	if err := r.InstallWebhookChart(ctx, watcherList, kyma, r.RestConfig); err != nil {
		updateErr := r.UpdateWatchersStatusesToError(ctx, logger, watcherList, "error configuring webhook chart resources")
		if updateErr == nil {
			return err
		}
		return updateErr
	}

	return r.UpdateWatchersStatusesToReady(ctx, logger, watcherList, "watcher configured successfully")
}

func (r *WatcherReconciler) UpdateWatchersStatusesToReady(ctx context.Context, logger logr.Logger, watcherList *v1alpha1.WatcherList, msg string) error {
	var err error
	for _, watcher := range watcherList.Items {
		if err = r.updateWatcherCRStatus(ctx, &watcher, v1alpha1.WatcherStateReady, msg); err != nil {
			logger.V(1).Error(err, "error updating watcher CR status", "watcher", watcher, "state", v1alpha1.WatcherStateReady)
		}
	}
	return err
}

func (r *WatcherReconciler) UpdateWatchersStatusesToError(ctx context.Context, logger logr.Logger, watcherList *v1alpha1.WatcherList, msg string) error {
	var err error
	for _, watcher := range watcherList.Items {
		if err = r.updateWatcherCRStatus(ctx, &watcher, v1alpha1.WatcherStateError, msg); err != nil {
			logger.V(1).Error(err, "error updating watcher CR status", "watcher", watcher, "state", v1alpha1.WatcherStateError)
		}
	}
	return err
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

func (r *WatcherReconciler) SetSKRChartManager() error {
	//nolint:goerr113
	if r.Config == nil || r.RestConfig == nil {
		return fmt.Errorf("watcher config is not set")
	}
	chartMgr, err := deploy.NewSKRChartManager(r.RestConfig, r.Config.WebhookChartPath,
		r.Config.SkrWebhookMemoryLimits, r.Config.SkrWebhookCPULimits)
	r.SKRChartManager = chartMgr
	return err
}
