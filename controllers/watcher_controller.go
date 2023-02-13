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

	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	watcherFinalizer = "operator.kyma-project.io/watcher"
)

var ErrRestConfigIsNotSet = errors.New("reconciler rest config is not set")

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
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrlLog.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("Reconciliation loop starting")

	watcherObj := &v1beta1.Watcher{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcherObj); err != nil {
		logger.V(log.DebugLevel).Info("Failed to get reconciliation object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !watcherObj.DeletionTimestamp.IsZero() {
		err := r.IstioClient.RemoveVirtualServiceConfigForCR(ctx, req.NamespacedName)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(watcherObj, watcherFinalizer)
		err = r.Update(ctx, watcherObj)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// check finalizer on native object
	if !controllerutil.ContainsFinalizer(watcherObj, watcherFinalizer) {
		if controllerutil.AddFinalizer(watcherObj, watcherFinalizer) {
			return ctrl.Result{}, r.Update(ctx, watcherObj)
		}
	}

	virtualService, err := r.IstioClient.GetVirtualService(ctx)
	if apierrors.IsNotFound(err) {
		if _, err := r.IstioClient.CreateVirtualService(ctx, watcherObj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create virtual service %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.IstioClient.UpdateVirtualServiceConfig(ctx, watcherObj, virtualService)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
}
