package kyma

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

type KymaDeletionReconciler struct {
	client.Client
	event.Event
	queue.RequeueIntervals
	SkrContextFactory remote.SkrContextProvider
	SKRWebhookManager *watcher.SKRWebhookManifestManager
	InKCPMode         bool
	IsManagedKyma     bool
	Metrics           *metrics.KymaMetrics
	RemoteCatalog     *remote.RemoteCatalog
}

func (r *KymaDeletionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			if err = r.handleOrphanedResourcesDeletion(ctx, req.Name); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: false}, nil
		}
		r.Metrics.RecordRequeueReason(metrics.KymaRetrieval, queue.UnexpectedRequeue)
		return ctrl.Result{}, fmt.Errorf("KymaController: %w", err)
	}

	if kyma.DeletionTimestamp.IsZero() {
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	}

	status.InitConditions(kyma, r.SyncKymaEnabled(kyma), r.WatcherEnabled(kyma))

	// if kyma.SkipReconciliation() {
	// 	logger.V(log.DebugLevel).Info("skipping reconciliation for Kyma: " + kyma.Name)
	// 	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	// }

	err := r.SkrContextFactory.Init(ctx, kyma.GetNamespacedName())
	if errors.Is(err, common.ErrAccessSecretNotFound) {
		return r.handleDeletedSkr(ctx, kyma)
	}

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
		return ctrl.Result{Requeue: true}, nil
	}

	res, err := r.handleDeletingState(ctx, kyma)
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

func (r *KymaDeletionReconciler) syncStatusToRemote(ctx context.Context, kcpKyma *v1beta2.Kyma) error {
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

func (r *KymaDeletionReconciler) fetchRemoteKyma(ctx context.Context, kcpKyma *v1beta2.Kyma) (*v1beta2.Kyma, error) {
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

func (r *KymaDeletionReconciler) IsKymaManaged() bool {
	return r.IsManagedKyma
}

func (r *KymaDeletionReconciler) updateStatus(ctx context.Context, kyma *v1beta2.Kyma,
	state shared.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		r.Event.Warning(kyma, patchStatusError, err)
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *KymaDeletionReconciler) UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma) {
	if err := r.Metrics.UpdateAll(kyma); err != nil {
		if metrics.IsMissingMetricsAnnotationOrLabel(err) {
			r.Event.Warning(kyma, metricsError, err)
		}
		logf.FromContext(ctx).V(log.DebugLevel).Info(fmt.Sprintf("error occurred while updating all metrics: %s", err))
	}
}

func (r *KymaDeletionReconciler) deleteRemoteKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	if r.SyncKymaEnabled(kyma) {
		skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
		if err != nil {
			return fmt.Errorf("failed to get skrContext: %w", err)
		}
		if err := skrContext.DeleteKyma(ctx); client.IgnoreNotFound(err) != nil {
			logf.FromContext(ctx).V(log.InfoLevel).Error(err, "Failed to be deleted remotely!")
			return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
		}
		logf.FromContext(ctx).V(log.InfoLevel).Info("Successfully deleted remotely!")
	}
	return nil
}

func (r *KymaDeletionReconciler) requeueWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result,
	error,
) {
	return ctrl.Result{Requeue: true}, r.updateStatusWithError(ctx, kyma, err)
}

func (r *KymaDeletionReconciler) updateStatusWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, shared.StateError, err.Error()); err != nil {
		r.Event.Warning(kyma, updateStatusError, err)
		return fmt.Errorf("error while updating status to %s: %w", shared.StateError, err)
	}
	return nil
}

func (r *KymaDeletionReconciler) WatcherEnabled(kyma *v1beta2.Kyma) bool {
	return r.SyncKymaEnabled(kyma) && r.SKRWebhookManager != nil
}

func (r *KymaDeletionReconciler) IsInKcp() bool {
	return r.InKCPMode
}

func (r *KymaDeletionReconciler) SyncKymaEnabled(kyma *v1beta2.Kyma) bool {
	if !r.InKCPMode {
		return false
	}
	return kyma.HasSyncLabelEnabled()
}

func (r *KymaDeletionReconciler) handleOrphanedResourcesDeletion(ctx context.Context, kymaName string) error {
	if err := r.SKRWebhookManager.RemoveKCPCertificate(ctx, kymaName); err != nil {
		return err
	}

	return nil
}

func (r *KymaDeletionReconciler) handleDeletedSkr(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logf.FromContext(ctx).Info("access secret not found for kyma, assuming already deleted cluster")
	r.cleanupMetrics(kyma.Name)
	r.removeAllFinalizers(kyma)

	if err := r.updateKyma(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.IntendedRequeue)
	return ctrl.Result{Requeue: true}, nil
}

func (r *KymaDeletionReconciler) removeAllFinalizers(kyma *v1beta2.Kyma) {
	for _, finalizer := range kyma.Finalizers {
		controllerutil.RemoveFinalizer(kyma, finalizer)
	}
}

func (r *KymaDeletionReconciler) updateKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.Update(ctx, kyma); err != nil {
		err = fmt.Errorf("error while updating kyma during deletion: %w", err)
		r.Event.Warning(kyma, updateSpecError, err)
		return err
	}

	return nil
}

func (r *KymaDeletionReconciler) handleDeletingState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).V(log.InfoLevel)

	if r.WatcherEnabled(kyma) {
		if err := r.SKRWebhookManager.Remove(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		}
		r.SKRWebhookManager.WatcherMetrics.CleanupMetrics(kyma.Name)
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
	return ctrl.Result{Requeue: true}, nil
}

func (r *KymaDeletionReconciler) cleanupManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) error {
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

func (r *KymaDeletionReconciler) deleteManifests(ctx context.Context, manifests []v1beta2.Manifest) error {
	for i := range manifests {
		if err := r.Delete(ctx, &manifests[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("error while trying to delete manifest: %w", err)
		}
	}
	return nil
}

func (r *KymaDeletionReconciler) relatedManifestCRsAreDeleted(manifests []v1beta2.Manifest) bool {
	return len(manifests) == 0
}

func (r *KymaDeletionReconciler) getRelatedManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) ([]v1beta2.Manifest,
	error,
) {
	manifestList := &v1beta2.ManifestList{}
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kyma.Name})
	if err := r.List(ctx, manifestList,
		&client.ListOptions{LabelSelector: labelSelector}); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get related manifests, %w", err)
	}

	return manifestList.Items, nil
}

func (r *KymaDeletionReconciler) cleanupMetrics(kymaName string) {
	r.Metrics.CleanupMetrics(kymaName)
}
