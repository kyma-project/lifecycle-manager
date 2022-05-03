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
	"golang.org/x/mod/semver"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
)

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;onEvent
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kyma object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("reconciling " + req.NamespacedName.String())
	kyma := operatorv1alpha1.Kyma{}

	if err := r.Get(ctx, req.NamespacedName, &kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() {
		return ctrl.Result{Requeue: true}, nil
	}

	err := r.onCreateOrUpdate(ctx, &kyma)
	if err != nil {
		return ctrl.Result{}, err // retry
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Kyma{}).
		Complete(r)
}

func (r *KymaReconciler) onCreateOrUpdate(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	kyma = kyma.DeepCopy()

	if (kyma.Status.State == operatorv1alpha1.KymaStateReady || kyma.Status.State == operatorv1alpha1.KymaStateError) && kyma.Status.
		ObservedGeneration == kyma.
		ObjectMeta.Generation {
		logger.Info("skipping reconciliation for " + kyma.Name + ", already reconciled!")
		return nil
	}

	if kyma.Status.State == operatorv1alpha1.KymaStateProcessing {
		logger.Info("processing " + kyma.Name)
		ready := true
		for _, condition := range kyma.Status.Conditions {
			if condition.Type == operatorv1alpha1.ConditionTypeReady && condition.Status != operatorv1alpha1.ConditionStatusTrue {
				ready = false
				break
			}
		}
		if ready {
			logger.Info(fmt.Sprintf("reconciliation of %s finished!", kyma.Name))
			r.Recorder.Event(kyma, "Normal", "ReconciliationSuccess", fmt.Sprintf("Reconciliation finished!"))
			kyma.Status.ObservedGeneration = kyma.ObjectMeta.Generation
			kyma.Status.State = operatorv1alpha1.KymaStateReady
			return r.updateKymaStatus(ctx, kyma)
		}
	}

	kyma.Status.State = operatorv1alpha1.KymaStateProcessing
	kyma.Status.ObservedGeneration = kyma.ObjectMeta.Generation

	//THIS IS JUST STUB CODE
	if len(kyma.Status.Conditions) == 0 {
		logger.Info(fmt.Sprintf("fresh kyma installation %s, adding reconciliation conditions!", kyma.Name))
		kyma.Status.Conditions = append(kyma.Status.Conditions, operatorv1alpha1.KymaCondition{
			Type:               operatorv1alpha1.ConditionTypeReady,
			Status:             operatorv1alpha1.ConditionStatusFalse,
			Message:            "reconciliation has to be finished",
			Reason:             "reconciliation triggered",
			LastTransitionTime: &v1.Time{Time: time.Now()},
		})
		r.Recorder.Event(kyma, "Normal", "Updated", fmt.Sprintf("Reconciliation started"))
		return r.updateKymaStatus(ctx, kyma)
	} else {
		return r.reconcileKymaForRelease(ctx, kyma)
	}
}

func (r *KymaReconciler) reconcileKymaForRelease(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	kyma.Status.Conditions[0].Status = operatorv1alpha1.ConditionStatusTrue
	oldRelease, newRelease := kyma.Status.ActiveRelease, kyma.Spec.Release
	kyma.Status.ActiveRelease = newRelease

	compared := semver.Compare(oldRelease, newRelease)
	if compared < 0 {
		if oldRelease == "" {
			r.Recorder.Event(kyma, "Normal", "ReconciliationUpgrade", fmt.Sprintf("Initial Installation: %s", newRelease))
		} else {
			r.Recorder.Event(kyma, "Normal", "ReconciliationUpgrade", fmt.Sprintf("Upgrade from %s to %s",
				oldRelease, newRelease))
		}
	} else if compared > 0 {
		r.Recorder.Event(kyma, "Normal", "ReconciliationDowngrade", fmt.Sprintf("Downgrade from %s to %s",
			oldRelease, newRelease))
	} else {
		r.Recorder.Event(kyma, "Normal", "ReconciliationUpdate", fmt.Sprintf("Update Active Release %s",
			kyma.Spec.Release))
	}

	return r.updateKymaStatus(ctx, kyma)
}

func (r *KymaReconciler) updateKyma(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Update(ctx, kyma)
	})
}

func (r *KymaReconciler) updateKymaStatus(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Status().Update(ctx, kyma)
	})
}
