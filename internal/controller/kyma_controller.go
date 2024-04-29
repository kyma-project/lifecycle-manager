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

package controller

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

type (
	EventReasonError string
)

var ErrManifestsStillExist = errors.New("manifests still exist")

const (
	moduleReconciliationError EventReasonError = "ModuleReconciliationError"
	metricsError              EventReasonError = "MetricsError"
	updateSpecError           EventReasonError = "UpdateSpecError"
	updateStatusError         EventReasonError = "UpdateStatusError"
)

type KymaReconciler struct {
	client.Client
	record.EventRecorder
	queue.RequeueIntervals
	DescriptorProvider  *provider.CachedDescriptorProvider
	SyncRemoteCrds      remote.SyncCrdsUseCase
	SKRWebhookManager   *watcher.SKRWebhookManifestManager
	KcpRestConfig       *rest.Config
	RemoteClientCache   *remote.ClientCache
	InKCPMode           bool
	RemoteSyncNamespace string
	IsManagedKyma       bool
	Metrics             *metrics.KymaMetrics
}

// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;update;patch;watch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;create;update;delete;patch;watch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions/status,verbs=update

func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Kyma reconciliation started")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{Requeue: false}, nil
		}
		r.Metrics.RecordRequeueReason(metrics.KymaRetrieval, queue.UnexpectedRequeue)
		return ctrl.Result{}, fmt.Errorf("KymaController: %w", err)
	}

	status.InitConditions(kyma, r.SyncKymaEnabled(kyma), r.WatcherEnabled(kyma))

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info("skipping reconciliation for Kyma: " + kyma.Name)
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	}

	ctx, err := r.getSyncedContext(ctx, kyma)

	if !kyma.DeletionTimestamp.IsZero() && errors.Is(err, remote.ErrAccessSecretNotFound) {
		logger.Info("access secret not found for kyma, assuming already deleted cluster")
		r.Metrics.CleanupMetrics(kyma.Name)
		r.removeAllFinalizers(kyma)

		if err := r.updateKyma(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.UnexpectedRequeue)
			return ctrl.Result{}, err
		}
		r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.IntendedRequeue)
		return ctrl.Result{Requeue: true}, nil
	}

	// Prevent ETCD load bursts during secret rotation
	if apierrors.IsUnauthorized(err) {
		r.deleteRemoteClientCache(ctx, kyma)
		r.Metrics.RecordRequeueReason(metrics.KymaUnauthorized, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, err)
	}

	if err != nil {
		r.deleteRemoteClientCache(ctx, kyma)
		r.Metrics.RecordRequeueReason(metrics.SyncContextRetrieval, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, err)
	}

	return r.reconcile(ctx, kyma)
}

func (r *KymaReconciler) deleteRemoteClientCache(ctx context.Context, kyma *v1beta2.Kyma) {
	logger := logf.FromContext(ctx)
	logger.Info("connection refused, assuming connection is invalid and resetting cache-entry for kyma")
	r.RemoteClientCache.Delete(client.ObjectKeyFromObject(kyma))
}

// getSyncedContext returns either the original context (in case Syncing is disabled) or initiates a sync-context
// with a remote client and returns that context instead.
// In case of failure, original context should be returned.
func (r *KymaReconciler) getSyncedContext(ctx context.Context, kyma *v1beta2.Kyma) (context.Context, error) {
	if !r.SyncKymaEnabled(kyma) {
		return ctx, nil
	}

	remoteClient := remote.NewClientWithConfig(r.Client, r.KcpRestConfig)
	ctxWithSync, err := remote.InitializeSyncContext(ctx, kyma,
		r.RemoteSyncNamespace, remoteClient, r.RemoteClientCache)
	if err != nil {
		return ctx, err
	}

	return ctxWithSync, nil
}

func (r *KymaReconciler) reconcile(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != shared.StateDeleting {
		if err := r.deleteRemoteKyma(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.RemoteKymaDeletion, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, err)
		}
		if err := r.updateStatus(ctx, kyma, shared.StateDeleting, "waiting for modules to be deleted"); err != nil {
			r.Metrics.RecordRequeueReason(metrics.StatusUpdateToDeleting, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma,
				fmt.Errorf("could not update kyma status after triggering deletion: %w", err))
		}
		r.Metrics.RecordRequeueReason(metrics.StatusUpdateToDeleting, queue.IntendedRequeue)
		return ctrl.Result{Requeue: true}, nil
	}

	if needsUpdate := kyma.EnsureLabelsAndFinalizers(); needsUpdate {
		if err := r.Update(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.LabelsAndFinalizersUpdate, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, fmt.Errorf("failed to update kyma after finalizer check: %w", err))
		}
		r.Metrics.RecordRequeueReason(metrics.LabelsAndFinalizersUpdate, queue.IntendedRequeue)
		return ctrl.Result{Requeue: true}, nil
	}

	if r.SyncKymaEnabled(kyma) {
		updateKymaRequired, err := r.syncCrdsAndUpdateKymaAnnotations(ctx, kyma)
		if err != nil {
			r.Metrics.RecordRequeueReason(metrics.CrdsSync, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, fmt.Errorf("could not sync CRDs: %w", err))
		}
		if updateKymaRequired {
			if err := r.Update(ctx, kyma); err != nil {
				r.Metrics.RecordRequeueReason(metrics.CrdAnnotationsUpdate, queue.UnexpectedRequeue)
				return r.requeueWithError(ctx, kyma, fmt.Errorf("could not update kyma annotations: %w", err))
			}
			r.Metrics.RecordRequeueReason(metrics.CrdAnnotationsUpdate, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		// update the control-plane kyma with the changes to the spec of the remote Kyma
		if err := r.replaceSpecFromRemote(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.SpecReplacementFromRemote, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, fmt.Errorf("could not replace control plane kyma spec"+
				" with remote kyma spec: %w", err))
		}
	}

	res, err := r.processKymaState(ctx, kyma)
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.ProcessingKymaState, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}

	if r.SyncKymaEnabled(kyma) {
		// update the remote kyma with the state of the control plane
		if err := r.syncStatusToRemote(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.StatusSyncToRemote, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, fmt.Errorf("could not synchronize remote kyma status: %w", err))
		}
	}

	return res, err
}

func (r *KymaReconciler) syncCrdsAndUpdateKymaAnnotations(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get syncContext: %w", err)
	}
	updateRequired, err := r.SyncRemoteCrds.Execute(
		ctx, kyma, syncContext.RuntimeClient, syncContext.ControlPlaneClient)
	if err != nil {
		return false, err
	}

	return updateRequired, nil
}

func (r *KymaReconciler) deleteRemoteKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx).V(log.InfoLevel)
	if r.SyncKymaEnabled(kyma) {
		if err := remote.DeleteRemotelySyncedKyma(ctx, r.RemoteSyncNamespace); client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to be deleted remotely!")
			return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
		}
		logger.Info("Successfully deleted remotely!")
	}
	return nil
}

func (r *KymaReconciler) requeueWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result, error) {
	return ctrl.Result{Requeue: true}, r.updateStatusWithError(ctx, kyma, err)
}

func (r *KymaReconciler) enqueueWarningEvent(kyma *v1beta2.Kyma, reason EventReasonError, err error) {
	r.Event(kyma, "Warning", string(reason), err.Error())
}

func (r *KymaReconciler) fetchRemoteKyma(ctx context.Context, controlPlaneKyma *v1beta2.Kyma) (*v1beta2.Kyma, error) {
	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get syncContext: %w", err)
	}
	remoteKyma, err := syncContext.CreateOrFetchRemoteKyma(ctx, controlPlaneKyma, r.RemoteSyncNamespace)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			return nil, err
		}
		return nil, fmt.Errorf("could not create or fetch remote kyma: %w", err)
	}
	return remoteKyma, nil
}

// syncStatusToRemote updates the status of a remote copy of given Kyma instance.
func (r *KymaReconciler) syncStatusToRemote(ctx context.Context, controlPlaneKyma *v1beta2.Kyma) error {
	remoteKyma, err := r.fetchRemoteKyma(ctx, controlPlaneKyma)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			// remote kyma not found because it's deleted, should not continue
			return nil
		}
		return err
	}

	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}
	if err := syncContext.SynchronizeRemoteKyma(ctx, controlPlaneKyma, remoteKyma); err != nil {
		return fmt.Errorf("sync run failure: %w", err)
	}
	return nil
}

// replaceSpecFromRemote replaces the spec from control-lane Kyma with the remote Kyma spec as single source of truth.
func (r *KymaReconciler) replaceSpecFromRemote(
	ctx context.Context, controlPlaneKyma *v1beta2.Kyma,
) error {
	remoteKyma, err := r.fetchRemoteKyma(ctx, controlPlaneKyma)
	if err != nil {
		if errors.Is(err, remote.ErrNotFoundAndKCPKymaUnderDeleting) {
			// remote kyma not found because it's deleted, should not continue
			return nil
		}
		return err
	}
	remote.ReplaceModules(controlPlaneKyma, remoteKyma)
	return nil
}

func (r *KymaReconciler) processKymaState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	switch kyma.Status.State {
	case "":
		return r.handleInitialState(ctx, kyma)
	case shared.StateProcessing:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateDeleting:
		return r.handleDeletingState(ctx, kyma)
	case shared.StateError:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateReady, shared.StateWarning:
		return r.handleProcessingState(ctx, kyma)
	}

	return ctrl.Result{Requeue: false}, nil
}

func (r *KymaReconciler) handleInitialState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	if err := r.updateStatus(ctx, kyma, shared.StateProcessing, "started processing"); err != nil {
		r.Metrics.RecordRequeueReason(metrics.InitialStateHandling, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.InitialStateHandling, queue.IntendedRequeue)
	return ctrl.Result{Requeue: true}, nil
}

func (r *KymaReconciler) handleProcessingState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
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
	if r.SyncKymaEnabled(kyma) {
		errGroup.Go(func() error {
			if err := r.syncModuleCatalog(ctx, kyma); err != nil {
				r.Metrics.RecordRequeueReason(metrics.ModuleCatalogSync, queue.UnexpectedRequeue)
				kyma.UpdateCondition(v1beta2.ConditionTypeModuleCatalog, apimetav1.ConditionFalse)
				return fmt.Errorf("could not synchronize remote module catalog: %w", err)
			}
			kyma.UpdateCondition(v1beta2.ConditionTypeModuleCatalog, apimetav1.ConditionTrue)
			return nil
		})
	}

	if r.WatcherEnabled(kyma) {
		errGroup.Go(func() error {
			if err := r.SKRWebhookManager.Install(ctx, kyma); err != nil {
				r.Metrics.RecordRequeueReason(metrics.SkrWebhookResourcesInstallation, queue.UnexpectedRequeue)
				if errors.Is(err, &watcher.CertificateNotReadyError{}) {
					kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, apimetav1.ConditionFalse)
					return nil
				}
				return err
			}
			kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, apimetav1.ConditionTrue)
			return nil
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

	return ctrl.Result{RequeueAfter: requeueInterval},
		r.updateStatus(ctx, kyma, state, "waiting for all modules to become ready")
}

func (r *KymaReconciler) handleDeletingState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).V(log.InfoLevel)

	if r.WatcherEnabled(kyma) {
		if err := r.SKRWebhookManager.Remove(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		}
		r.SKRWebhookManager.WatcherMetrics.CleanupMetrics(kyma.Name)
	}

	if r.SyncKymaEnabled(kyma) {
		if err := remote.NewRemoteCatalogFromKyma(r.RemoteSyncNamespace).Delete(ctx); err != nil {
			err = fmt.Errorf("could not delete remote module catalog: %w", err)
			r.Metrics.RecordRequeueReason(metrics.RemoteModuleCatalogDeletion, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, err)
		}

		r.RemoteClientCache.Delete(client.ObjectKeyFromObject(kyma))
		if err := remote.RemoveFinalizersFromRemoteKyma(ctx, r.RemoteSyncNamespace); client.IgnoreNotFound(err) != nil {
			r.Metrics.RecordRequeueReason(metrics.FinalizersRemovalFromRemoteKyma, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, err)
		}

		logger.Info("removed remote finalizers")
	}

	if err := r.cleanupManifestCRs(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.CleanupManifestCrs, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}

	r.Metrics.CleanupMetrics(kyma.Name)

	controllerutil.RemoveFinalizer(kyma, shared.KymaFinalizer)

	if err := r.updateKyma(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.KymaDeletion, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.KymaDeletion, queue.IntendedRequeue)
	return ctrl.Result{Requeue: true}, nil
}

func (r *KymaReconciler) cleanupManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) error {
	relatedManifests, err := r.getRelatedManifestCRs(ctx, kyma)
	if err != nil {
		return fmt.Errorf("error while trying to get manifests: %w", err)
	}

	if r.relatedManifestCRsAreDeleted(relatedManifests) {
		return nil
	}

	if err = r.deleteManifests(ctx, relatedManifests); err != nil {
		return fmt.Errorf("error while trying to delete manifests: %w", err)
	}
	return ErrManifestsStillExist
}

func (r *KymaReconciler) deleteManifests(ctx context.Context, manifests []v1beta2.Manifest) error {
	for i := range manifests {
		if err := r.Delete(ctx, &manifests[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("error while trying to delete manifest: %w", err)
		}
	}
	return nil
}

func (r *KymaReconciler) getRelatedManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) ([]v1beta2.Manifest, error) {
	manifestList := &v1beta2.ManifestList{}
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kyma.Name})
	if err := r.List(ctx, manifestList,
		&client.ListOptions{LabelSelector: labelSelector}); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get related manifests, %w", err)
	}

	return manifestList.Items, nil
}

func (r *KymaReconciler) relatedManifestCRsAreDeleted(manifests []v1beta2.Manifest) bool {
	return len(manifests) == 0
}

func (r *KymaReconciler) removeAllFinalizers(kyma *v1beta2.Kyma) {
	for _, finalizer := range kyma.Finalizers {
		controllerutil.RemoveFinalizer(kyma, finalizer)
	}
}

func (r *KymaReconciler) updateKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.Update(ctx, kyma); err != nil {
		err = fmt.Errorf("error while updating kyma during deletion: %w", err)
		r.enqueueWarningEvent(kyma, updateSpecError, err)
		return err
	}

	return nil
}

func (r *KymaReconciler) reconcileManifests(ctx context.Context, kyma *v1beta2.Kyma) error {
	modules, err := r.GenerateModulesFromTemplate(ctx, kyma)
	if err != nil {
		return fmt.Errorf("error while fetching modules during processing: %w", err)
	}

	runner := sync.New(r)

	if err := runner.ReconcileManifests(ctx, kyma, modules); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	runner.SyncModuleStatus(ctx, kyma, modules, r.Metrics)
	// If module get removed from kyma, the module deletion happens here.
	if err := r.DeleteNoLongerExistingModules(ctx, kyma); err != nil {
		return fmt.Errorf("error while syncing conditions during deleting non exists modules: %w", err)
	}
	return nil
}

func (r *KymaReconciler) syncModuleCatalog(ctx context.Context, kyma *v1beta2.Kyma) error {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := r.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("could not aggregate module templates for module catalog sync: %w", err)
	}

	var modulesToSync []v1beta2.ModuleTemplate
	for _, mt := range moduleTemplateList.Items {
		if mt.SyncEnabled(kyma.IsBeta(), kyma.IsInternal()) {
			modulesToSync = append(modulesToSync, mt)
		}
	}

	if err := remote.NewRemoteCatalogFromKyma(r.RemoteSyncNamespace).CreateOrUpdate(ctx, modulesToSync); err != nil {
		return fmt.Errorf("could not synchronize remote module catalog: %w", err)
	}

	return nil
}

func (r *KymaReconciler) updateStatus(ctx context.Context, kyma *v1beta2.Kyma,
	state shared.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		r.Event(kyma, "Warning", "PatchStatus", err.Error())
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *KymaReconciler) updateStatusWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, shared.StateError, err.Error()); err != nil {
		r.enqueueWarningEvent(kyma, updateStatusError, err)
		return fmt.Errorf("error while updating status to %s: %w", shared.StateError, err)
	}
	return nil
}

func (r *KymaReconciler) GenerateModulesFromTemplate(ctx context.Context, kyma *v1beta2.Kyma) (common.Modules, error) {
	lookup := templatelookup.NewTemplateLookup(client.Reader(r), r.DescriptorProvider)
	templates := lookup.GetRegularTemplates(ctx, kyma)
	for _, template := range templates {
		if template.Err != nil {
			r.enqueueWarningEvent(kyma, moduleReconciliationError, template.Err)
		}
	}
	parser := parse.NewParser(r.Client, r.DescriptorProvider, r.InKCPMode, r.RemoteSyncNamespace)
	return parser.GenerateModulesFromTemplates(kyma, templates), nil
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

func (r *KymaReconciler) UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma) {
	if err := r.Metrics.UpdateAll(kyma); err != nil {
		if metrics.IsMissingMetricsAnnotationOrLabel(err) {
			r.enqueueWarningEvent(kyma, metricsError, err)
		}
		logf.FromContext(ctx).V(log.DebugLevel).Info(fmt.Sprintf("error occurred while updating all metrics: %s", err))
	}
}

func (r *KymaReconciler) WatcherEnabled(kyma *v1beta2.Kyma) bool {
	return r.SyncKymaEnabled(kyma) && r.SKRWebhookManager != nil
}

func (r *KymaReconciler) IsInKcp() bool {
	return r.InKCPMode
}

func (r *KymaReconciler) SyncKymaEnabled(kyma *v1beta2.Kyma) bool {
	if !r.InKCPMode {
		return false
	}
	return kyma.HasSyncLabelEnabled()
}

func (r *KymaReconciler) IsKymaManaged() bool {
	return r.IsManagedKyma
}
