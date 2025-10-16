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

package kyma

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

type InstallationReconciler struct {
	client.Client
	event.Event
	queue.RequeueIntervals

	SkrContextFactory    remote.SkrContextProvider
	DescriptorProvider   *provider.CachedDescriptorProvider
	SyncRemoteCrds       remote.SyncCrdsUseCase
	ModulesStatusHandler ModuleStatusHandler
	SKRWebhookManager    SKRWebhookManager
	RemoteSyncNamespace  string
	IsManagedKyma        bool
	Metrics              *metrics.KymaMetrics
	RemoteCatalog        *remote.RemoteCatalog
	TemplateLookup       *templatelookup.TemplateLookup
}

func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Kyma installation reconciliation started")

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{}, nil
		}
		r.Metrics.RecordRequeueReason(metrics.KymaRetrieval, queue.UnexpectedRequeue)
		return ctrl.Result{}, fmt.Errorf("KymaInstallationController: %w", err)
	}

	// If Kyma is under deletion, let the deletion controller handle it
	if !kyma.DeletionTimestamp.IsZero() {
		logger.V(log.DebugLevel).Info("Kyma under deletion, leaving to deletion controller")
		return ctrl.Result{}, nil
	}

	if err := r.UpdateModuleTemplatesIfNeeded(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("KymaInstallationController: %w", err)
	}

	status.InitConditions(kyma, r.WatcherEnabled())

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info("skipping reconciliation for Kyma: " + kyma.Name)
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	err := r.SkrContextFactory.Init(ctx, kyma.GetNamespacedName())
	skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.SyncContextRetrieval, queue.UnexpectedRequeue)
		setModuleStatusesToError(kyma, err.Error())
		return r.requeueWithError(ctx, kyma, err)
	}

	err = skrContext.CreateKymaNamespace(ctx)
	if apierrors.IsUnauthorized(err) {
		r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
		logger.Info("connection refused, assuming connection is invalid and resetting cache-entry for kyma")
		r.Metrics.RecordRequeueReason(metrics.KymaUnauthorized, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, err)
	}
	if err != nil {
		r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
		r.Metrics.RecordRequeueReason(metrics.SyncContextRetrieval, queue.UnexpectedRequeue)
		setModuleStatusesToError(kyma, util.NestedErrorMessage(err))
		return r.requeueWithError(ctx, kyma, err)
	}

	return r.reconcileInstallation(ctx, kyma)
}

func (r *InstallationReconciler) reconcileInstallation(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	if needsUpdate := kyma.EnsureLabelsAndFinalizers(); needsUpdate {
		if err := r.Update(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.LabelsAndFinalizersUpdate, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, fmt.Errorf("failed to update kyma after finalizer check: %w", err))
		}
		r.Metrics.RecordRequeueReason(metrics.LabelsAndFinalizersUpdate, queue.IntendedRequeue)
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, nil
	}

	updateRequired, err := r.SyncRemoteCrds.Execute(ctx, kyma)
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.CrdsSync, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, fmt.Errorf("could not sync CRDs: %w", err))
	}
	if updateRequired {
		if err := r.Update(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.CrdAnnotationsUpdate, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, fmt.Errorf("could not update kyma annotations: %w", err))
		}
		r.Metrics.RecordRequeueReason(metrics.CrdAnnotationsUpdate, queue.IntendedRequeue)
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, nil
	}
	// update the control-plane kyma with the changes to the spec of the remote Kyma
	if err = r.replaceSpecFromRemote(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.SpecReplacementFromRemote, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, fmt.Errorf("could not replace control plane kyma spec"+
			" with remote kyma spec: %w", err))
	}

	res, err := r.processKymaState(ctx, kyma)
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.ProcessingKymaState, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}

	if err := r.syncStatusToRemote(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.StatusSyncToRemote, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, fmt.Errorf("could not synchronize remote kyma status: %w", err))
	}

	return res, err
}

func (r *InstallationReconciler) fetchRemoteKyma(ctx context.Context, kcpKyma *v1beta2.Kyma) (*v1beta2.Kyma, error) {
	syncContext, err := r.SkrContextFactory.Get(kcpKyma.GetNamespacedName())
	if err != nil {
		return nil, fmt.Errorf("failed to get syncContext: %w", err)
	}
	remoteKyma, err := syncContext.CreateOrFetchKyma(ctx, r.Client, kcpKyma)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			return nil, err
		}
		return nil, fmt.Errorf("could not create or fetch remote kyma: %w", err)
	}
	return remoteKyma, nil
}

// syncStatusToRemote updates the status of a remote copy of given Kyma instance.
func (r *InstallationReconciler) syncStatusToRemote(ctx context.Context, kcpKyma *v1beta2.Kyma) error {
	remoteKyma, err := r.fetchRemoteKyma(ctx, kcpKyma)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			// remote kyma not found because it's deleted, should not continue
			return nil
		}
		return err
	}

	skrContext, err := r.SkrContextFactory.Get(kcpKyma.GetNamespacedName())
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}

	if err := skrContext.SynchronizeKymaMetadata(ctx, kcpKyma, remoteKyma); err != nil {
		return fmt.Errorf("failed to sync SKR Kyma CR Metadata: %w", err)
	}

	if err := skrContext.SynchronizeKymaStatus(ctx, kcpKyma, remoteKyma); err != nil {
		return fmt.Errorf("failed to sync SKR Kyma CR Status: %w", err)
	}

	return nil
}

// replaceSpecFromRemote replaces the spec from control-lane Kyma with the remote Kyma spec as single source of truth.
func (r *InstallationReconciler) replaceSpecFromRemote(ctx context.Context, controlPlaneKyma *v1beta2.Kyma) error {
	remoteKyma, err := r.fetchRemoteKyma(ctx, controlPlaneKyma)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			// remote kyma not found because it's deleted, should not continue
			return nil
		}
		return err
	}

	remote.ReplaceSpec(controlPlaneKyma, remoteKyma)

	if err := r.ValidateDefaultChannel(controlPlaneKyma); err != nil {
		return err
	}

	return nil
}

func (r *InstallationReconciler) processKymaState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	switch kyma.Status.State {
	case "":
		return r.handleInitialState(ctx, kyma)
	case shared.StateProcessing:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateError:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateReady, shared.StateWarning:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateUnmanaged:
		return ctrl.Result{}, nil // no requeue of invalid state
	}

	return ctrl.Result{}, nil
}

func (r *InstallationReconciler) handleInitialState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	if err := r.updateStatus(ctx, kyma, shared.StateProcessing, "started processing"); err != nil {
		r.Metrics.RecordRequeueReason(metrics.InitialStateHandling, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.InitialStateHandling, queue.IntendedRequeue)
	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, nil
}

func (r *InstallationReconciler) handleProcessingState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	var errGroup errgroup.Group
	errGroup.Go(func() error {
		err := r.reconcileManifests(ctx, kyma)
		if err != nil {
			r.Metrics.RecordRequeueReason(metrics.ReconcileManifests, queue.UnexpectedRequeue)
			return fmt.Errorf("could not reconciling manifest: %w", err)
		}
		if kyma.AllModulesReady() {
			kyma.UpdateCondition(v1beta2.ConditionTypeModules, apimetav1.ConditionTrue)
		} else {
			kyma.UpdateCondition(v1beta2.ConditionTypeModules, apimetav1.ConditionFalse)
		}
		return nil
	})

	errGroup.Go(func() error {
		if err := r.RemoteCatalog.SyncModuleCatalog(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.ModuleCatalogSync, queue.UnexpectedRequeue)
			kyma.UpdateCondition(v1beta2.ConditionTypeModuleCatalog, apimetav1.ConditionFalse)
			return fmt.Errorf("failed to synchronize remote module catalog: %w", err)
		}
		kyma.UpdateCondition(v1beta2.ConditionTypeModuleCatalog, apimetav1.ConditionTrue)
		return nil
	})

	if r.WatcherEnabled() {
		errGroup.Go(func() error {
			if err := r.SKRWebhookManager.Reconcile(ctx, kyma); err != nil {
				r.Metrics.RecordRequeueReason(metrics.SkrWebhookResourcesInstallation, queue.UnexpectedRequeue)
				kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, apimetav1.ConditionFalse)
				if errors.Is(err, watcher.ErrSkrCertificateNotReady) {
					return nil
				}
				return err
			}
			skrClient, _ := r.SkrContextFactory.Get(client.ObjectKeyFromObject(kyma))
			return checkSKRWebhookReadiness(ctx, skrClient, kyma)
		})
	}

	if err := errGroup.Wait(); err != nil {
		return r.requeueWithError(ctx, kyma, err)
	}

	state := kyma.DetermineState()
	requeueInterval := queue.DetermineRequeueInterval(state, r.RequeueIntervals)
	if state == shared.StateReady {
		const msg = "kyma is ready"
		if kyma.Status.State != shared.StateReady {
			logger.Info(msg)
		}
		return ctrl.Result{RequeueAfter: requeueInterval}, r.updateStatus(ctx, kyma, state, msg)
	}
	err := r.updateStatus(ctx, kyma, state, "waiting for all modules to become ready")
	if err != nil {
		return ctrl.Result{}, err
	}
	if state == shared.StateError {
		// Requeue with a new Error in case of Kyma error state, to enable rate limiting for that error.
		return ctrl.Result{}, ErrKymaInErrorState
	}
	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func (r *InstallationReconciler) reconcileManifests(ctx context.Context, kyma *v1beta2.Kyma) error {
	templates := r.TemplateLookup.GetRegularTemplates(ctx, kyma)
	prsr := parser.NewParser(r.Client, r.DescriptorProvider, r.RemoteSyncNamespace)
	modules := prsr.GenerateModulesFromTemplates(kyma, templates)

	runner := sync.New(r)
	if err := runner.ReconcileManifests(ctx, kyma, modules); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	err := r.ModulesStatusHandler.UpdateModuleStatuses(ctx, kyma, modules)
	if err != nil {
		return fmt.Errorf("failed to update module statuses: %w", err)
	}

	// If module get removed from kyma, the module deletion happens here.
	if err := r.DeleteNoLongerExistingModules(ctx, kyma); err != nil {
		return fmt.Errorf("error while syncing conditions during deleting non exists modules: %w", err)
	}
	return nil
}

// Helper methods for installation controller
func (r *InstallationReconciler) ValidateDefaultChannel(kyma *v1beta2.Kyma) error {
	if shared.NoneChannel.Equals(kyma.Spec.Channel) {
		return fmt.Errorf("%w: value \"none\" is not allowed in spec.channel", ErrInvalidKymaSpec)
	}
	return nil
}

func (r *InstallationReconciler) DeleteNoLongerExistingModules(ctx context.Context, kyma *v1beta2.Kyma) error {
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

func (r *InstallationReconciler) UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma) {
	if err := r.Metrics.UpdateAll(kyma); err != nil {
		if metrics.IsMissingMetricsAnnotationOrLabel(err) {
			r.Event.Warning(kyma, metricsError, err)
		}
		logf.FromContext(ctx).V(log.DebugLevel).Info(fmt.Sprintf("error occurred while updating all metrics: %s", err))
	}
}

func (r *InstallationReconciler) WatcherEnabled() bool {
	return r.SKRWebhookManager != nil
}

func (r *InstallationReconciler) IsKymaManaged() bool {
	return r.IsManagedKyma
}

func (r *InstallationReconciler) GetModuleTemplateList(ctx context.Context) (*v1beta2.ModuleTemplateList, error) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := r.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("could not aggregate module templates for module catalog sync: %w", err)
	}

	return moduleTemplateList, nil
}

func (r *InstallationReconciler) UpdateModuleTemplatesIfNeeded(ctx context.Context) error {
	moduleTemplateList, err := r.GetModuleTemplateList(ctx)
	if err != nil {
		return err
	}

	for _, mt := range moduleTemplateList.Items {
		if needUpdateForMandatoryModuleLabel(mt) {
			if err = r.Update(ctx, &mt); err != nil {
				return fmt.Errorf("failed to update ModuleTemplate, %w", err)
			}
		}
	}

	return nil
}

func (r *InstallationReconciler) requeueWithError(ctx context.Context,
	kyma *v1beta2.Kyma,
	err error,
) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, r.updateStatusWithError(ctx, kyma, err)
}

func (r *InstallationReconciler) updateStatus(ctx context.Context, kyma *v1beta2.Kyma,
	state shared.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		r.Event.Warning(kyma, patchStatusError, err)
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *InstallationReconciler) updateStatusWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, shared.StateError, err.Error()); err != nil {
		r.Event.Warning(kyma, updateStatusError, err)
		return fmt.Errorf("error while updating status to %s: %w", shared.StateError, err)
	}
	return nil
}

func (r *InstallationReconciler) deleteManifest(ctx context.Context, trackedManifest *v1beta2.TrackingObject) error {
	manifest := apimetav1.PartialObjectMetadata{}
	manifest.SetGroupVersionKind(trackedManifest.GroupVersionKind())
	manifest.SetNamespace(trackedManifest.GetNamespace())
	manifest.SetName(trackedManifest.GetName())

	err := r.Delete(ctx, &manifest, &client.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed delete manifest crd: %w", err)
	}
	return nil
}
