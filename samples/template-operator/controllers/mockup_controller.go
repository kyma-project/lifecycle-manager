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
	"github.com/kyma-project/kyma-operator/samples/template-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MockupReconciler reconciles a Mockup object
type MockupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=component.kyma-project.io,resources=mockups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=mockups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=mockups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Mockup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *MockupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// TODO(user): your logic here

	// check if Mockup resource exists
	mockupResource := &v1alpha1.Mockup{}
	if err := r.Get(ctx, req.NamespacedName, mockupResource); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch mockupResource.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, mockupResource)
	case v1alpha1.MockupStateProcessing:
		return ctrl.Result{}, r.HandleProcessingState(ctx, mockupResource)
	case v1alpha1.MockupStateDeleting:
		return ctrl.Result{}, r.HandleDeletingState(ctx, mockupResource)
	case v1alpha1.MockupStateError:
		return ctrl.Result{}, r.HandleErrorState(ctx, mockupResource)
	case v1alpha1.MockupStateReady:
		return ctrl.Result{}, r.HandleReadyState(ctx, mockupResource)
	}

	return ctrl.Result{}, nil
}

func (r *MockupReconciler) HandleInitialState(ctx context.Context, mockupResource *v1alpha1.Mockup) error {
	// TODO: initial logic here

	// Example: Set to Processing state
	mockupResource.Status.State = v1alpha1.MockupStateProcessing
	return r.Client.Status().Update(ctx, mockupResource)
}

func (r *MockupReconciler) HandleProcessingState(ctx context.Context, mockupResource *v1alpha1.Mockup) error {
	// TODO: processing logic here

	// Example: If Processing succeeds, set to Ready state
	mockupResource.Status.State = v1alpha1.MockupStateReady
	return r.Client.Status().Update(ctx, mockupResource)
}

func (r *MockupReconciler) HandleDeletingState(ctx context.Context, mockupResource *v1alpha1.Mockup) error {
	// TODO: deletion logic here

	// Example: If Deleting state, remove Finalizers
	mockupResource.Finalizers = nil
	return r.Client.Update(ctx, mockupResource)
}

func (r *MockupReconciler) HandleErrorState(ctx context.Context, mockupResource *v1alpha1.Mockup) error {
	// TODO: error logic here

	// Example: If Error state, set state to Processing
	mockupResource.Status.State = v1alpha1.MockupStateProcessing
	return r.Client.Status().Update(ctx, mockupResource)
}

func (r *MockupReconciler) HandleReadyState(_ context.Context, _ *v1alpha1.Mockup) error {
	// TODO: ready logic here

	// Example: If Ready state, check consistency of deployed module
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MockupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Mockup{}).
		Complete(r)
}
