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

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type DeletionReconciler struct {
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

func (r *DeletionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			if err = r.deleteOrphanedCertificate(ctx, req.Name); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		r.Metrics.RecordRequeueReason(metrics.KymaRetrieval, queue.UnexpectedRequeue)
		return ctrl.Result{}, fmt.Errorf("kyma DeletionReconciler: %w", err)
	}

	// Only handle deletion cases - if no deletion timestamp, let the installation controller handle it
	if kyma.DeletionTimestamp.IsZero() {
		logger.V(log.DebugLevel).Info("Kyma not under deletion, skipping deletion controller")
		return ctrl.Result{}, nil
	}

	logger.V(log.DebugLevel).Info("Kyma deletion reconciliation started")

	status.InitConditions(kyma, r.WatcherEnabled()) // check what conditions are used in the deletion use-case
	
	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info("skipping deletion reconciliation for Kyma: " + kyma.Name)
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	err := r.SkrContextFactory.Init(ctx, kyma.GetNamespacedName())
	if errors.Is(err, accessmanager.ErrAccessSecretNotFound) {
		return r.handleDeletedSkr(ctx, kyma)
	}

	skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.SyncContextRetrieval, queue.UnexpectedRequeue)
		setModuleStatusesToError(kyma, err.Error())
		return r.requeueWithError(ctx, kyma, err)
	}

	return r.reconcileDeletion(ctx, kyma, skrContext)
}

func (r *DeletionReconciler) reconcileDeletion(ctx context.Context,
	kyma *v1beta2.Kyma,
	_ *remote.SkrContext,
) (ctrl.Result, error) {
	if kyma.Status.State != shared.StateDeleting {
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
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, nil
	}

	return r.handleDeletingState(ctx, kyma)
}

func (r *DeletionReconciler) handleDeletedSkr(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logf.FromContext(ctx).Info("access secret not found for kyma, assuming already deleted cluster")
	if err := r.cleanupManifestCRs(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.CleanupManifestCrs, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.cleanupMetrics(kyma.Name)
	r.removeAllFinalizers(kyma)

	if err := r.updateKyma(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.IntendedRequeue)
	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, nil
}

func (r *DeletionReconciler) deleteRemoteKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}
	if err := skrContext.DeleteKyma(ctx); client.IgnoreNotFound(err) != nil {
		logf.FromContext(ctx).V(log.InfoLevel).Error(err, "Failed to be deleted remotely!")
		return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
	}
	logf.FromContext(ctx).V(log.InfoLevel).Info("Successfully deleted remotely!")

	return nil
}

func (r *DeletionReconciler) handleDeletingState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).V(log.InfoLevel)

	if r.WatcherEnabled() {
		if err := r.SKRWebhookManager.Remove(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.RemoteCatalog.Delete(ctx, kyma.GetNamespacedName()); err != nil {
		err = fmt.Errorf("failed to delete remote module catalog: %w", err)
		r.Metrics.RecordRequeueReason(metrics.RemoteModuleCatalogDeletion, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, err)
	}
	skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get skrContext: %w", err)
	}

	r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
	if err = skrContext.RemoveFinalizersFromKyma(ctx); client.IgnoreNotFound(err) != nil {
		r.Metrics.RecordRequeueReason(metrics.FinalizersRemovalFromRemoteKyma, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, err)
	}

	logger.Info("removed remote finalizers")

	if err := r.cleanupManifestCRs(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.CleanupManifestCrs, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}

	r.cleanupMetrics(kyma.Name)
	controllerutil.RemoveFinalizer(kyma, shared.KymaFinalizer)

	if err := r.updateKyma(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.KymaDeletion, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.KymaDeletion, queue.IntendedRequeue)
	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, nil
}

// Methods required by HelperClient interface
func (r *DeletionReconciler) WatcherEnabled() bool {
	return r.SKRWebhookManager != nil
}

func (r *DeletionReconciler) IsKymaManaged() bool {
	return r.IsManagedKyma
}

func (r *DeletionReconciler) UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma) {
	if err := r.Metrics.UpdateAll(kyma); err != nil {
		if metrics.IsMissingMetricsAnnotationOrLabel(err) {
			r.Event.Warning(kyma, metricsError, err)
		}
		logf.FromContext(ctx).V(log.DebugLevel).Info(fmt.Sprintf("error occurred while updating all metrics: %s", err))
	}
}

// Helper methods for deletion controller
func (r *DeletionReconciler) cleanupMetrics(kymaName string) {
	r.Metrics.CleanupMetrics(kymaName)
}

func (r *DeletionReconciler) cleanupManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) error {
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

func (r *DeletionReconciler) deleteManifests(ctx context.Context, manifests []v1beta2.Manifest) error {
	for i := range manifests {
		if err := r.Delete(ctx, &manifests[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("error while trying to delete manifest: %w", err)
		}
	}
	return nil
}

func (r *DeletionReconciler) getRelatedManifestCRs(ctx context.Context,
	kyma *v1beta2.Kyma,
) ([]v1beta2.Manifest, error) {
	manifestList := &v1beta2.ManifestList{}
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kyma.Name})
	if err := r.List(ctx, manifestList,
		&client.ListOptions{LabelSelector: labelSelector}); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get related manifests, %w", err)
	}

	return manifestList.Items, nil
}

func (r *DeletionReconciler) relatedManifestCRsAreDeleted(manifests []v1beta2.Manifest) bool {
	return len(manifests) == 0
}

func (r *DeletionReconciler) removeAllFinalizers(kyma *v1beta2.Kyma) {
	for _, finalizer := range kyma.Finalizers {
		controllerutil.RemoveFinalizer(kyma, finalizer)
	}
}

func (r *DeletionReconciler) updateKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.Update(ctx, kyma); err != nil {
		err = fmt.Errorf("error while updating kyma during deletion: %w", err)
		r.Event.Warning(kyma, updateSpecError, err)
		return err
	}

	return nil
}

func (r *DeletionReconciler) updateStatus(ctx context.Context, kyma *v1beta2.Kyma,
	state shared.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		r.Event.Warning(kyma, patchStatusError, err)
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *DeletionReconciler) updateStatusWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, shared.StateError, err.Error()); err != nil {
		r.Event.Warning(kyma, updateStatusError, err)
		return fmt.Errorf("error while updating status to %s: %w", shared.StateError, err)
	}
	return nil
}

func (r *DeletionReconciler) requeueWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, r.updateStatusWithError(ctx, kyma, err)
}

func (r *DeletionReconciler) deleteManifest(ctx context.Context, trackedManifest *v1beta2.TrackingObject) error {
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

func (r *DeletionReconciler) deleteOrphanedCertificate(ctx context.Context, kymaName string) error {
	if !r.WatcherEnabled() {
		return nil
	}
	return r.SKRWebhookManager.RemoveSkrCertificate(ctx, kymaName)
}

func (r *DeletionReconciler) DeleteNoLongerExistingModules(ctx context.Context, kyma *v1beta2.Kyma) error {
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
