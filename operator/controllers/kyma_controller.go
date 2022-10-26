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

	"github.com/kyma-project/lifecycle-manager/operator/pkg/catalog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/channel"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/module/parse"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/status"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type EventErrorType string

const ModuleReconciliationError EventErrorType = "ModuleReconciliationError"

type RequeueIntervals struct {
	Success time.Duration
	Failure time.Duration
	Waiting time.Duration
}

// KymaReconciler reconciles a Kyma object.
type KymaReconciler struct {
	client.Client
	record.EventRecorder
	RequeueIntervals
	signature.VerificationSettings
	RemoteClientCache *remote.ClientCache
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
	logger.Info("reconciling modules", "resource", req.NamespacedName.String())

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	// check if kyma resource exists
	kyma := &v1alpha1.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")

		return ctrl.Result{}, client.IgnoreNotFound(err) //nolint:wrapcheck
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != v1alpha1.StateDeleting {
		if err := r.TriggerKymaDeletion(ctx, kyma); err != nil {
			return r.CtrlErr(ctx, kyma, err)
		}

		// if the status is not yet set to deleting, also update the status
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
		syncContext, err := remote.InitializeKymaSynchronizationContext(ctx, r.Client, kyma, r.RemoteClientCache)
		if err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf("remote sync initialization failed: %w", err))
		}
		if err := r.syncRemote(ctx, syncContext); err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf("could not synchronize remote kyma: %w", err))
		}
		if err := r.syncModuleCatalog(ctx, syncContext); err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf("could not synchronize remote module catalog: %w", err))
		}
		if syncContext.RequiresStatusUpdateInControlPlane() {
			return ctrl.Result{}, r.Status().Update(ctx, kyma)
		}
	}

	// state handling
	return r.stateHandling(ctx, kyma)
}

func (r *KymaReconciler) CtrlErr(ctx context.Context, kyma *v1alpha1.Kyma, err error) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure},
		r.UpdateStatusWithEventFromErr(ctx, kyma, v1alpha1.StateError, err)
}

// synchronizeRemote replaces the given pointer to the Kyma Instance with an instance that contains the merged
// specification of the Control Plane and the Runtime.
func (r *KymaReconciler) syncRemote(ctx context.Context, syncContext *remote.KymaSynchronizationContext) error {
	kyma := syncContext.ControlPlaneKyma
	if kyma.Status.State == v1alpha1.StateDeleting {
		return nil
	}
	remoteKyma, err := syncContext.CreateOrFetchRemoteKyma(ctx)
	if err != nil {
		return fmt.Errorf("could not create or fetch remote kyma: %w", err)
	}
	if err := syncContext.SynchronizeRemoteKyma(ctx, remoteKyma); err != nil {
		return fmt.Errorf("sync run failure: %w", err)
	}
	syncContext.ReplaceWithVirtualKyma(kyma, remoteKyma)

	return nil
}

func (r *KymaReconciler) syncModuleCatalog(ctx context.Context, syncContext *remote.KymaSynchronizationContext) error {
	if !syncContext.ControlPlaneKyma.Spec.Sync.ModuleCatalog {
		return nil
	}

	if !syncContext.ControlPlaneKyma.ContainsCondition(v1alpha1.ConditionTypeReady,
		v1alpha1.ConditionReasonModuleCatalogIsReady) {
		syncContext.ControlPlaneKyma.UpdateCondition(v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionFalse)
		syncContext.RequireStatusUpdateInControlPlane()
	}

	moduleTemplateList := &v1alpha1.ModuleTemplateList{}
	if err := r.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("could not aggregate module templates for module catalog sync: %w", err)
	}

	force := true

	if err := catalog.NewRemoteCatalog(
		syncContext, catalog.Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: "catalog-sync", Force: &force},
		},
	).CreateOrUpdate(ctx, moduleTemplateList); err != nil {
		return fmt.Errorf("could not synchronize remote module catalog: %w", err)
	}

	if !syncContext.ControlPlaneKyma.ContainsCondition(v1alpha1.ConditionTypeReady,
		v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionTrue) {
		syncContext.ControlPlaneKyma.UpdateCondition(v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionTrue)
		syncContext.RequireStatusUpdateInControlPlane()
	}

	return nil
}

func (r *KymaReconciler) stateHandling(ctx context.Context, kyma *v1alpha1.Kyma) (ctrl.Result, error) {
	log.FromContext(ctx).Info("syncing state", "state", string(kyma.Status.State))
	switch kyma.Status.State {
	case "":

		return ctrl.Result{}, r.HandleInitialState(ctx, kyma)
	case v1alpha1.StateProcessing:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, r.HandleProcessingState(ctx, kyma)
	case v1alpha1.StateDeleting:
		if dependentsDeleting, err := r.HandleDeletingState(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		} else if dependentsDeleting {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Waiting}, nil
		}
	case v1alpha1.StateError:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Waiting}, r.HandleProcessingState(ctx, kyma)
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

	var err error
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

	kyma.SyncConditionsWithModuleStates()
	// set ready condition if applicable
	if kyma.AreAllConditionsReadyForKyma() && kyma.Status.State != v1alpha1.StateReady {
		message := fmt.Sprintf("reconciliation of %s finished!", kyma.Name)
		logger.Info(message)
		r.Event(kyma, "Normal", "ReconciliationSuccess", message)

		return r.UpdateStatusWithEvent(ctx, kyma, v1alpha1.StateReady, message)
	}

	// if the ready condition is not applicable, but we changed the conditions, we still need to issue an update
	if statusUpdateRequiredFromModuleSync ||
		statusUpdateRequiredFromModuleStatusSync ||
		statusUpdateRequiredFromDeletion {
		if err := r.UpdateStatusWithEvent(ctx, kyma, v1alpha1.StateProcessing, "updating component conditions"); err != nil {
			return fmt.Errorf("error while updating status for condition change: %w", err)
		}
		return nil
	}

	return nil
}

func (r *KymaReconciler) HandleDeletingState(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := log.FromContext(ctx)

	if kyma.Spec.Sync.Enabled {
		syncContext, err := remote.InitializeKymaSynchronizationContext(ctx, r.Client, kyma, r.RemoteClientCache)
		if err != nil {
			return false, fmt.Errorf("remote sync initialization failed: %w", err)
		}
		force := true
		if err := catalog.NewRemoteCatalog(
			syncContext, catalog.Settings{
				SSAPatchOptions: &client.PatchOptions{FieldManager: "catalog-sync", Force: &force},
			},
		).Delete(ctx); err != nil {
			return false, fmt.Errorf("could not delete remote module catalog: %w", err)
		}
		r.RemoteClientCache.Del(client.ObjectKeyFromObject(kyma))
		if err := remote.RemoveFinalizerFromRemoteKyma(ctx, kyma, syncContext); client.IgnoreNotFound(err) != nil {
			return false, fmt.Errorf("error while trying to remove finalizer from remote: %w", err)
		}
		logger.Info("removed remote finalizer",
			"resource", client.ObjectKeyFromObject(kyma))
	}

	controllerutil.RemoveFinalizer(kyma, v1alpha1.Finalizer)

	if err := r.Update(ctx, kyma); err != nil {
		return false, fmt.Errorf("error while trying to udpate kyma during deletion: %w", err)
	}

	return false, nil
}

func (r *KymaReconciler) TriggerKymaDeletion(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	namespacedName := types.NamespacedName{
		Namespace: kyma.GetNamespace(),
		Name:      kyma.GetName(),
	}.String()
	if kyma.Spec.Sync.Enabled {
		if err := remote.DeleteRemotelySyncedKyma(
			ctx, r.Client, r.RemoteClientCache, kyma,
		); client.IgnoreNotFound(err) != nil {
			logger.Info(namespacedName + " could not be deleted remotely!")
			return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
		}
		logger.Info(namespacedName + " got deleted remotely!")
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
	module := unstructured.Unstructured{}
	module.SetNamespace(moduleStatus.Namespace)
	module.SetName(moduleStatus.Name)
	module.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   moduleStatus.TemplateInfo.GroupVersionKind.Group,
		Version: moduleStatus.TemplateInfo.GroupVersionKind.Version,
		Kind:    moduleStatus.TemplateInfo.GroupVersionKind.Kind,
	})
	return r.Delete(ctx, &module, &client.DeleteOptions{})
}
