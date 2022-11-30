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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/catalog"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"github.com/kyma-project/lifecycle-manager/pkg/deploy"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	manifestV1alpha1 "github.com/kyma-project/module-manager/api/v1alpha1"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type EventErrorType string

const (
	ModuleReconciliationError EventErrorType = "ModuleReconciliationError"
	SyncContextError          EventErrorType = "SyncContextError"
	DeletionError             EventErrorType = "DeletionError"
)

type RequeueIntervals struct {
	Success time.Duration
}

// KymaReconciler reconciles a Kyma object.
type KymaReconciler struct {
	client.Client
	record.EventRecorder
	RequeueIntervals
	signature.VerificationSettings
	RemoteClientCache *remote.ClientCache
	deploy.SKRWebhookChartManager
	KcpRestConfig *rest.Config
}

//nolint:lll
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling modules")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	// check if kyma resource exists
	kyma := &v1alpha1.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info("Deleted successfully!")

		return ctrl.Result{}, client.IgnoreNotFound(err) //nolint:wrapcheck
	}

	if kyma.Spec.Sync.Enabled {
		var err error
		if ctx, err = remote.InitializeSyncContext(ctx, kyma, r.Client, r.RemoteClientCache); err != nil {
			err := fmt.Errorf("initializing sync context failed: %w", err)
			r.Event(kyma, "Warning", string(SyncContextError), err.Error())
			return r.CtrlErr(ctx, kyma, err)
		}
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != v1alpha1.StateDeleting {
		if err := r.TriggerKymaDeletion(ctx, kyma); err != nil {
			return r.CtrlErr(ctx, kyma, err)
		}

		// if the status is not yet set to deleting, also update the status of the control-plane
		// in the next sync cycle
		if err := status.Helper(r).UpdateStatusForExistingModules(
			ctx, kyma, v1alpha1.StateDeleting,
		); err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf(
				"could not update kyma status after triggering deletion: %w", err))
		}
		return ctrl.Result{}, nil
	}

	// check finalizer
	if kyma.CheckLabelsAndFinalizers() {
		if err := r.Update(ctx, kyma); err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf("could not update kyma after finalizer check: %w", err))
		}
		return ctrl.Result{}, nil
	}

	// create a remote synchronization context, and update the remote kyma with the state of the control plane
	if kyma.Spec.Sync.Enabled {
		if err := r.syncRemoteKymaSpecAndStatus(ctx, kyma); err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf("could not synchronize remote kyma: %w", err))
		}
	}

	// state handling
	return r.stateHandling(ctx, kyma)
}

func (r *KymaReconciler) CtrlErr(ctx context.Context, kyma *v1alpha1.Kyma, err error) (ctrl.Result, error) {
	return ctrl.Result{Requeue: true},
		r.UpdateStatusWithEventFromErr(ctx, kyma, v1alpha1.StateError, err)
}

// synchronizeRemote replaces the given pointer to the Kyma Instance with an instance that contains the merged
// specification of the Control Plane and the Runtime.
func (r *KymaReconciler) syncRemoteKymaSpecAndStatus(
	ctx context.Context, controlPlaneKyma *v1alpha1.Kyma,
) error {
	syncContext := remote.SyncContextFromContext(ctx)

	remoteKyma, err := syncContext.CreateOrFetchRemoteKyma(ctx, controlPlaneKyma)
	if err != nil {
		return fmt.Errorf("could not create or fetch remote kyma: %w", err)
	}
	if err := syncContext.SynchronizeRemoteKyma(ctx, controlPlaneKyma, remoteKyma); err != nil {
		return fmt.Errorf("sync run failure: %w", err)
	}
	syncContext.ReplaceWithVirtualKyma(controlPlaneKyma, remoteKyma)

	return nil
}

func (r *KymaReconciler) syncModuleCatalog(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	if !kyma.Spec.Sync.Enabled || !kyma.Spec.Sync.ModuleCatalog {
		return false, nil
	}

	requiresStatusUpdate := false

	if !kyma.ContainsCondition(
		v1alpha1.ConditionTypeReady,
		v1alpha1.ConditionReasonModuleCatalogIsReady,
	) {
		kyma.UpdateCondition(v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionFalse)
		requiresStatusUpdate = true
	}

	moduleTemplateList := &v1alpha1.ModuleTemplateList{}
	if err := r.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
		return false, fmt.Errorf("could not aggregate module templates for module catalog sync: %w", err)
	}

	if err := catalog.NewRemoteCatalogFromKyma(kyma).CreateOrUpdate(ctx, moduleTemplateList); err != nil {
		return false, fmt.Errorf("could not synchronize remote module catalog: %w", err)
	}

	if !kyma.ContainsCondition(
		v1alpha1.ConditionTypeReady,
		v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionTrue) {
		kyma.UpdateCondition(v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionTrue)
		requiresStatusUpdate = true
	}

	return requiresStatusUpdate, nil
}

func (r *KymaReconciler) stateHandling(ctx context.Context, kyma *v1alpha1.Kyma) (ctrl.Result, error) {
	log.FromContext(ctx).Info("syncing state", "state", string(kyma.Status.State))
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, kyma)
	case v1alpha1.StateProcessing:
		return ctrl.Result{Requeue: true}, r.HandleProcessingState(ctx, kyma)
	case v1alpha1.StateDeleting:
		if dependentsDeleting, err := r.HandleDeletingState(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		} else if dependentsDeleting {
			return ctrl.Result{Requeue: true}, nil
		}
	case v1alpha1.StateError:
		return ctrl.Result{Requeue: true}, r.HandleProcessingState(ctx, kyma)
	case v1alpha1.StateReady:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, r.HandleProcessingState(ctx, kyma)
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) HandleInitialState(ctx context.Context, kyma *v1alpha1.Kyma) error {
	return r.UpdateStatusWithEvent(ctx, kyma, v1alpha1.StateProcessing, "initial state")
}

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := log.FromContext(ctx)

	statusUpdateRequiredFromCatalog, err := r.syncModuleCatalog(ctx, kyma)
	if err != nil {
		return r.UpdateStatusWithEventFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("could not synchronize remote module catalog: %w", err))
	}

	var modules common.Modules
	// these are the actual modules
	modules, err = r.GenerateModulesFromTemplate(ctx, kyma)
	if err != nil {
		return r.UpdateStatusWithEventFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("error while fetching modules during processing: %w", err))
	}

	runner := sync.New(r)

	statusUpdateRequiredFromModuleSync, err := runner.Sync(ctx, kyma, modules)
	if err != nil {
		return r.UpdateStatusWithEventFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("sync failed: %w", err))
	}

	statusUpdateRequiredFromModuleStatusSync := runner.SyncModuleStatus(ctx, kyma, modules)

	// If module get removed from kyma, the module deletion happens here.
	statusUpdateRequiredFromDeletion, err := r.DeleteNoLongerExistingModules(ctx, kyma)
	if err != nil {
		return r.UpdateStatusWithEventFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("error while syncing conditions during deleting non exists modules: %w", err))
	}
	statusUpdateRequiredFromSKRWebhookSync := false
	if kyma.Spec.Sync.Enabled {
		if statusUpdateRequiredFromSKRWebhookSync, err = r.InstallWebhookChart(ctx, kyma,
			r.RemoteClientCache, r.Client); err != nil {
			kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionFalse)
			return err
		}
	}
	kyma.SyncConditionsWithModuleStates()

	isStatusUpdateRequired := statusUpdateRequiredFromModuleSync || statusUpdateRequiredFromModuleStatusSync ||
		statusUpdateRequiredFromDeletion || statusUpdateRequiredFromSKRWebhookSync || statusUpdateRequiredFromCatalog
	// if the ready condition is not applicable, but we changed the conditions, we still need to issue an update
	if isStatusUpdateRequired {
		if err := r.UpdateStatusWithEvent(ctx, kyma, v1alpha1.StateProcessing, "updating component conditions"); err != nil {
			return fmt.Errorf("error while updating status for condition change: %w", err)
		}
		return nil
	}

	// set ready condition if applicable
	if kyma.AreAllConditionsReadyForKyma() && kyma.Status.State != v1alpha1.StateReady {
		const message = "Reconciliation finished!"
		logger.Info(message)
		r.Event(kyma, "Normal", "ReconciliationSuccess", message)
		return r.UpdateStatusWithEvent(ctx, kyma, v1alpha1.StateReady, message)
	}
	return nil
}

func (r *KymaReconciler) HandleDeletingState(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := log.FromContext(ctx)

	if kyma.Spec.Sync.Enabled {
		if err := r.RemoveWebhookChart(ctx, kyma); err != nil {
			// here we expect that an error is normal and means we have to try again if it didn't work
			r.Event(kyma, "Normal", "WebhookChartRemoval", err.Error())
			return true, nil
		}

		if err := catalog.NewRemoteCatalogFromKyma(kyma).Delete(ctx); err != nil {
			err := fmt.Errorf("could not delete remote module catalog: %w", err)
			r.Event(kyma, "Warning", string(DeletionError), err.Error())
			return false, err
		}

		r.RemoteClientCache.Del(client.ObjectKeyFromObject(kyma))
		if err := remote.RemoveFinalizerFromRemoteKyma(ctx, kyma); client.IgnoreNotFound(err) != nil {
			err := fmt.Errorf("error while trying to remove finalizer from remote: %w", err)
			r.Event(kyma, "Warning", string(DeletionError), err.Error())
			return false, err
		}

		logger.Info("removed remote finalizer")
	}

	controllerutil.RemoveFinalizer(kyma, v1alpha1.Finalizer)

	if err := r.Update(ctx, kyma); err != nil {
		err := fmt.Errorf("error while trying to udpate kyma during deletion: %w", err)
		r.Event(kyma, "Warning", string(DeletionError), err.Error())
		return false, err
	}

	return false, nil
}

func (r *KymaReconciler) TriggerKymaDeletion(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := log.FromContext(ctx)

	if kyma.Spec.Sync.Enabled {
		if err := remote.DeleteRemotelySyncedKyma(ctx, kyma); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to be deleted remotely!")
			return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
		}
		logger.Info("Successfully deleted remotely!")
	}
	return nil
}

func (r *KymaReconciler) UpdateStatusWithEvent(
	ctx context.Context, kyma *v1alpha1.Kyma, state v1alpha1.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state); err != nil {
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	r.Event(kyma, "Normal", "StatusUpdate", message)
	return nil
}

func (r *KymaReconciler) UpdateStatusWithEventFromErr(
	ctx context.Context, kyma *v1alpha1.Kyma, state v1alpha1.State, err error,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state); err != nil {
		return fmt.Errorf("error while updating status to %s: %w", state, err)
	}
	r.Event(kyma, "Warning", string(ModuleReconciliationError), err.Error())
	return nil
}

func (r *KymaReconciler) GenerateModulesFromTemplate(ctx context.Context, kyma *v1alpha1.Kyma) (common.Modules, error) {
	// fetch templates
	templates, err := channel.GetTemplates(ctx, r, kyma)
	if err != nil {
		return nil, fmt.Errorf("templates could not be fetched: %w", err)
	}

	verification, err := r.VerificationSettings.NewVerification(ctx, kyma.GetNamespace())
	if err != nil {
		return nil, err
	}

	// these are the actual modules
	modules, err := parse.GenerateModulesFromTemplates(kyma, templates, verification)
	if err != nil {
		return nil, fmt.Errorf("cannot generate modules: %w", err)
	}

	return modules, nil
}

func (r *KymaReconciler) DeleteNoLongerExistingModules(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	moduleStatus := kyma.GetNoLongerExistingModuleStatus()
	var err error
	if len(moduleStatus) == 0 {
		return false, nil
	}
	for i := range moduleStatus {
		moduleStatus := moduleStatus[i]
		err = r.deleteModule(ctx, moduleStatus)
	}

	if client.IgnoreNotFound(err) != nil {
		return true, fmt.Errorf("error deleting module %w", err)
	}
	return true, nil
}

func (r *KymaReconciler) deleteModule(ctx context.Context, moduleStatus *v1alpha1.ModuleStatus) error {
	manifest := manifestV1alpha1.Manifest{}
	manifest.SetNamespace(moduleStatus.Namespace)
	manifest.SetName(moduleStatus.Name)
	return r.Delete(ctx, &manifest, &client.DeleteOptions{})
}
