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
	"strings"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"golang.org/x/sync/errgroup"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
	modulesync "github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	SKRWebhookManager        watcher.SKRWebhookManager
	KcpRestConfig            *rest.Config
	RemoteClientCache        *remote.ClientCache
	ComponentDescriptorCache *ocmextensions.ComponentDescriptorCache
	InKCPMode                bool
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
//+kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch
//+kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;create;update;delete;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrlLog.FromContext(ctx)
	logger.V(log.InfoLevel).Info("reconciling")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)
	// check if kyma resource exists
	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info("Deleted successfully!")

		return ctrl.Result{}, client.IgnoreNotFound(err) //nolint:wrapcheck
	}

	status.InitConditions(kyma, kyma.SyncEnabled(), r.WatcherEnabled(kyma))

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info("kyma gets skipped because of label")
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	}

	if kyma.SyncEnabled() {
		var err error
		if ctx, err = remote.InitializeSyncContext(ctx, kyma,
			remote.NewClientWithConfig(r.Client, r.KcpRestConfig), r.RemoteClientCache); err != nil {
			err := fmt.Errorf("initializing sync context failed: %w", err)
			r.Event(kyma, "Warning", string(SyncContextError), err.Error())
			return r.CtrlErr(ctx, kyma, err)
		}
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != v1beta2.StateDeleting {
		return r.deleteKyma(ctx, kyma)
	}

	// check finalizer
	if kyma.CheckLabelsAndFinalizers() {
		if err := r.Update(ctx, kyma); err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf("could not update kyma after finalizer check: %w", err))
		}
		return ctrl.Result{}, nil
	}

	// update the remote kyma with the state of the control plane
	if kyma.SyncEnabled() {
		if err := r.syncRemoteKymaSpecAndStatus(ctx, kyma); err != nil {
			return r.CtrlErr(ctx, kyma, fmt.Errorf("could not synchronize remote kyma: %w", err))
		}
	}

	// state handling
	return r.stateHandling(ctx, kyma)
}

func (r *KymaReconciler) deleteKyma(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	if err := r.TriggerKymaDeletion(ctx, kyma); err != nil {
		return r.CtrlErr(ctx, kyma, err)
	}

	// if the status is not yet set to deleting, also update the status of the control-plane
	// in the next sync cycle
	if err := status.Helper(r).UpdateStatusForExistingModules(
		ctx, kyma, v1beta2.StateDeleting, "waiting for modules to be deleted",
	); err != nil {
		return r.CtrlErr(ctx, kyma, fmt.Errorf(
			"could not update kyma status after triggering deletion: %w", err))
	}
	return ctrl.Result{}, nil
}

func (r *KymaReconciler) CtrlErr(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result, error) {
	return ctrl.Result{Requeue: true},
		r.UpdateStatusWithEventFromErr(ctx, kyma, v1beta2.StateError, err)
}

// synchronizeRemote replaces the given pointer to the Kyma Instance with an instance that contains the merged
// specification of the Control Plane and the Runtime.
func (r *KymaReconciler) syncRemoteKymaSpecAndStatus(
	ctx context.Context, controlPlaneKyma *v1beta2.Kyma,
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

func (r *KymaReconciler) syncModuleCatalog(ctx context.Context, kyma *v1beta2.Kyma) error {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := r.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("could not aggregate module templates for module catalog sync: %w", err)
	}

	modulesToSync := []v1beta2.ModuleTemplate{}
	for _, mt := range moduleTemplateList.Items {
		if mt.SyncEnabled(kyma.IsBeta(), kyma.IsInternal()) {
			modulesToSync = append(modulesToSync, mt)
		}
	}

	if err := remote.NewRemoteCatalogFromKyma(kyma).CreateOrUpdate(ctx, modulesToSync, kyma); err != nil {
		return fmt.Errorf("could not synchronize remote module catalog: %w", err)
	}

	return nil
}

func (r *KymaReconciler) stateHandling(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.handleInitialState(ctx, kyma)
	case v1beta2.StateProcessing:
		return ctrl.Result{Requeue: true}, r.handleProcessingState(ctx, kyma)
	case v1beta2.StateDeleting:
		if dependentsDeleting, err := r.handleDeletingState(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		} else if dependentsDeleting {
			return ctrl.Result{Requeue: true}, nil
		}
	case v1beta2.StateError:
		return ctrl.Result{Requeue: true}, r.handleProcessingState(ctx, kyma)
	case v1beta2.StateReady:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, r.handleProcessingState(ctx, kyma)
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) handleInitialState(ctx context.Context, kyma *v1beta2.Kyma) error {
	return r.UpdateStatusWithEvent(ctx, kyma, v1beta2.StateProcessing, "started processing")
}

func (r *KymaReconciler) handleProcessingState(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := ctrlLog.FromContext(ctx)

	var errGroup errgroup.Group

	if kyma.SyncEnabled() {
		errGroup.Go(func() error { return r.syncModuleCatalogInParallel(ctx, kyma) })
	}

	errGroup.Go(func() error { return r.reconcileManifestsInParallel(ctx, kyma) })

	if r.WatcherEnabled(kyma) {
		errGroup.Go(func() error { return r.installWatcherInParallel(ctx, kyma) })
	}

	if err := errGroup.Wait(); err != nil {
		return err
	}

	// set ready condition if applicable
	state := kyma.DetermineState()

	if state == v1beta2.StateReady {
		const message = "kyma is ready"
		if kyma.Status.State != v1beta2.StateReady {
			logger.Info(message)
		}
		return r.UpdateStatus(ctx, kyma, state, message)
	}

	if err := r.UpdateStatus(ctx, kyma, state, "waiting for all modules to become ready"); err != nil {
		return fmt.Errorf("error while updating status for condition change: %w", err)
	}

	return nil
}

func (r *KymaReconciler) syncModuleCatalogInParallel(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.syncModuleCatalog(ctx, kyma); err != nil {
		return r.UpdateStatusWithEventFromErr(ctx, kyma, v1beta2.StateError,
			fmt.Errorf("could not synchronize remote module catalog: %w", err))
	}
	kyma.UpdateCondition(v1beta2.ConditionTypeModuleCatalog, metav1.ConditionTrue)
	return nil
}

func (r *KymaReconciler) reconcileManifestsInParallel(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.reconcileManifests(ctx, kyma); err != nil {
		return r.UpdateStatusWithEventFromErr(ctx, kyma, v1beta2.StateError, err)
	} else if kyma.AllModulesReady() {
		kyma.UpdateCondition(v1beta2.ConditionTypeModules, metav1.ConditionTrue)
	}
	return nil
}

func (r *KymaReconciler) installWatcherInParallel(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.SKRWebhookManager.Install(ctx, kyma); err != nil {
		if !errors.Is(err, &watcher.CertificateNotReadyError{}) {
			return r.UpdateStatusWithEventFromErr(ctx, kyma, v1beta2.StateError,
				fmt.Errorf("error while installing Watcher Webhook Chart: %w", err))
		}
	}
	kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, metav1.ConditionTrue)
	return nil
}

func (r *KymaReconciler) reconcileManifests(ctx context.Context, kyma *v1beta2.Kyma) error {
	// these are the actual modules
	modules, err := r.GenerateModulesFromTemplate(ctx, kyma)
	if err != nil {
		return fmt.Errorf("error while fetching modules during processing: %w", err)
	}

	runner := modulesync.New(r)

	if err := runner.ReconcileManifests(ctx, kyma, modules); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	runner.SyncModuleStatus(ctx, kyma, modules)
	// If module get removed from kyma, the module deletion happens here.

	if err := r.DeleteNoLongerExistingModules(ctx, kyma); err != nil {
		return fmt.Errorf("error while syncing conditions during deleting non exists modules: %w", err)
	}
	return nil
}

func (r *KymaReconciler) handleDeletingState(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	logger := ctrlLog.FromContext(ctx).V(log.InfoLevel)

	if r.WatcherEnabled(kyma) {
		if err := r.SKRWebhookManager.Remove(ctx, kyma); err != nil {
			// here we expect that an error is normal and means we have to try again if it didn't work
			r.Event(kyma, "Normal", "WebhookChartRemoval", err.Error())
			return true, nil
		}
	}

	if kyma.SyncEnabled() {
		if err := remote.NewRemoteCatalogFromKyma(kyma).Delete(ctx); err != nil {
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

	controllerutil.RemoveFinalizer(kyma, v1beta2.Finalizer)

	if err := r.Update(ctx, kyma); err != nil {
		err := fmt.Errorf("error while trying to udpate kyma during deletion: %w", err)
		r.Event(kyma, "Warning", string(DeletionError), err.Error())
		return false, err
	}

	return false, nil
}

func (r *KymaReconciler) TriggerKymaDeletion(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := ctrlLog.FromContext(ctx).V(log.InfoLevel)
	if kyma.SyncEnabled() {
		if err := remote.DeleteRemotelySyncedKyma(ctx, kyma); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to be deleted remotely!")
			return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
		}
		logger.Info("Successfully deleted remotely!")
	}
	return nil
}

func (r *KymaReconciler) UpdateStatus(
	ctx context.Context, kyma *v1beta2.Kyma, state v1beta2.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *KymaReconciler) UpdateStatusWithEvent(
	ctx context.Context, kyma *v1beta2.Kyma, state v1beta2.State, message string,
) error {
	if err := r.UpdateStatus(ctx, kyma, state, message); err != nil {
		return err
	}
	r.Event(kyma, "Normal", "StatusUpdate", message)
	return nil
}

func (r *KymaReconciler) UpdateStatusWithEventFromErr(
	ctx context.Context, kyma *v1beta2.Kyma, state v1beta2.State, err error,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, err.Error()); err != nil {
		return fmt.Errorf("error while updating status to %s: %w", state, err)
	}
	r.Event(kyma, "Warning", string(ModuleReconciliationError), err.Error())
	return nil
}

func (r *KymaReconciler) GenerateModulesFromTemplate(ctx context.Context, kyma *v1beta2.Kyma) (common.Modules, error) {
	templates := channel.GetTemplates(ctx, r, kyma)
	for _, template := range templates {
		if template.Err != nil {
			r.Event(kyma, "Warning", string(ModuleReconciliationError), template.Err.Error())
		}
	}

	verification, err := r.VerificationSettings.NewVerification(ctx, kyma.GetNamespace())
	if err != nil {
		return nil, err
	}
	parser := parse.NewParser(r.Client, r.ComponentDescriptorCache, r.InKCPMode)
	return parser.GenerateModulesFromTemplates(ctx, kyma, templates, verification), nil
}

func (r *KymaReconciler) DeleteNoLongerExistingModules(ctx context.Context, kyma *v1beta2.Kyma) error {
	moduleStatus := kyma.GetNoLongerExistingModuleStatus()
	var err error
	if len(moduleStatus) == 0 {
		return nil
	}
	for i := range moduleStatus {
		moduleStatus := moduleStatus[i]
		if moduleStatus.Manifest == nil {
			continue
		}
		err = r.deleteManifest(ctx, moduleStatus.Manifest)
	}

	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("error deleting module %w", err)
	}
	return nil
}

func (r *KymaReconciler) deleteManifest(ctx context.Context, trackedManifest *v1beta2.TrackingObject) error {
	manifest := metav1.PartialObjectMetadata{}
	manifest.SetGroupVersionKind(trackedManifest.GroupVersionKind())
	manifest.SetNamespace(trackedManifest.GetNamespace())
	manifest.SetName(trackedManifest.GetName())
	return r.Delete(ctx, &manifest, &client.DeleteOptions{})
}

// RecordKymaStatusMetrics updates prometheus metrics defined to track changes to the Kyma status.
func (r *KymaReconciler) RecordKymaStatusMetrics(ctx context.Context, kyma *v1beta2.Kyma) {
	logger := ctrlLog.FromContext(ctx).V(log.InfoLevel)
	shoot := ""
	shootFQDN, keyExists := kyma.Annotations[v1beta2.SKRDomainAnnotation]
	if keyExists {
		parts := strings.Split(shootFQDN, ".")
		minFqdnParts := 2
		if len(parts) > minFqdnParts {
			shoot = parts[0] // hostname
		}
	} else {
		logger.Info(fmt.Sprintf("expected annotation: %s not found when setting metric", v1beta2.SKRDomainAnnotation))
	}

	instanceID, keyExists := kyma.Labels[v1beta2.InstanceIDLabel]
	if !keyExists {
		logger.Info(fmt.Sprintf("expected label: %s not found when setting metric", v1beta2.InstanceIDLabel))
	}

	metrics.SetKymaStateGauge(kyma.Status.State, kyma.Name, shoot, instanceID)
	for _, moduleStatus := range kyma.Status.Modules {
		metrics.SetModuleStateGauge(moduleStatus.State, moduleStatus.Name, kyma.Name, shoot, instanceID)
	}
}

func (r *KymaReconciler) WatcherEnabled(kyma *v1beta2.Kyma) bool {
	if kyma.SyncEnabled() && r.SKRWebhookManager != nil {
		return true
	}
	return false
}

func (r *KymaReconciler) IsKymaManaged() bool {
	return r.InKCPMode
}
