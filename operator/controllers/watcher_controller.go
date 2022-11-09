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

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	watcherFinalizer = "operator.kyma-project.io/watcher"
)

var ErrRestConfigIsNotSet = errors.New("reconciler rest config is not set")

// WatcherReconciler reconciles a Watcher object.
type WatcherReconciler struct {
	client.Client
	*custom.IstioClient
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
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
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("Reconciliation loop starting")

	watcherObj := &v1alpha1.Watcher{}
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcherObj); err != nil {
		logger.Error(err, "Failed to get reconciliation object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !watcherObj.DeletionTimestamp.IsZero() {
		err := r.RemoveVirtualServiceConfigForCR(ctx, req.NamespacedName)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		controllerutil.RemoveFinalizer(watcherObj, watcherFinalizer)
		err = r.Update(ctx, watcherObj)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}

	// check finalizer on native object
	if !controllerutil.ContainsFinalizer(watcherObj, watcherFinalizer) {
		controllerutil.AddFinalizer(watcherObj, watcherFinalizer)
		err := r.Update(ctx, watcherObj)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}
	err := r.UpdateVirtualServiceConfig(ctx, watcherObj)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{Requeue: true}, nil
}
