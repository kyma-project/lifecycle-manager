package kyma

import (
	"context"
	"errors"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/service"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type KymaDeletionReconciler struct {
	event.Event
	queue.RequeueIntervals
	Metrics             *metrics.KymaMetrics
	SKRWebhookManager   *service.SKRWebhookManagerService
	RemoteCatalog       *service.RemoteCatalogService
	kymaService         *service.KymaService
	kymaSKRService      *service.KymaSKRService
	kymaDeletionService *service.KymaDeletionService
	skrContextService   *service.SKRContextService
}

func NewKymaDeletionReconciler(client client.Client, skrContextService *service.SKRContextService,
	event event.Event,
	requeueIntervals queue.RequeueIntervals, skrWebhookManager *service.SKRWebhookManagerService,
	inKCPMode, isManagedKyma bool, remoteSyncNamespace string, metrics *metrics.KymaMetrics,
) *KymaDeletionReconciler {
	kymaService := service.NewKymaService(client, inKCPMode, isManagedKyma)

	return &KymaDeletionReconciler{
		Event:            event,
		RequeueIntervals: requeueIntervals,
		Metrics:          metrics,
		kymaService:      kymaService,
		RemoteCatalog: service.NewRemoteCatalogFromKyma(client, skrContextService,
			remoteSyncNamespace),
		kymaSKRService:      service.NewKymaSKRService(client, skrContextService),
		kymaDeletionService: service.NewKymaDeletionService(client, skrWebhookManager, kymaService, metrics),
		skrContextService:   skrContextService,
	}
}

func (r *KymaDeletionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	kyma, err := r.kymaService.GetKyma(ctx, req.NamespacedName)
	if err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			if err = r.kymaDeletionService.HandleOrphanedResourcesDeletion(ctx, req.Name); err != nil {
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

	r.kymaService.InitConditions(kyma, r.WatcherEnabled())
	// if kyma.SkipReconciliation() {
	// 	logger.V(log.DebugLevel).Info("skipping reconciliation for Kyma: " + kyma.Name)
	// 	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	// }

	err = r.skrContextService.CacheContext(ctx, kyma.GetNamespacedName())
	if errors.Is(err, common.ErrAccessSecretNotFound) {
		return r.handleDeletedSkr(ctx, kyma)
	}

	if kyma.Status.State != shared.StateDeleting {
		if err := r.kymaDeletionService.DeleteRemoteKyma(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.RemoteKymaDeletion, queue.UnexpectedRequeue)
			return r.requeueWithError(ctx, kyma, err)
		}
		if err := r.kymaService.UpdateStatus(ctx, kyma, shared.StateDeleting,
			"waiting for modules to be deleted"); err != nil {
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

	if err := r.kymaSKRService.SyncStatusToRemote(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.StatusSyncToRemote, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, fmt.Errorf("could not synchronize remote kyma status: %w", err))
	}

	return res, err
}

func (r *KymaDeletionReconciler) requeueWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result,
	error,
) {
	return ctrl.Result{Requeue: true}, r.updateStatusWithError(ctx, kyma, err)
}

func (r *KymaDeletionReconciler) updateStatusWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) error {
	return r.kymaService.UpdateStatus(ctx, kyma, shared.StateError, err.Error())
}

func (r *KymaDeletionReconciler) WatcherEnabled() bool {
	return r.SKRWebhookManager != nil
}

func (r *KymaDeletionReconciler) handleDeletedSkr(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logf.FromContext(ctx).Info("access secret not found for kyma, assuming already deleted cluster")

	if err := r.kymaDeletionService.CleanupKyma(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.KymaUnderDeletionAndAccessSecretNotFound, queue.IntendedRequeue)
	return ctrl.Result{Requeue: true}, nil
}

func (r *KymaDeletionReconciler) handleDeletingState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).V(log.InfoLevel)

	if r.WatcherEnabled() {
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

	if err := r.kymaSKRService.DropKyma(ctx, kyma.GetNamespacedName()); err != nil {
		r.Metrics.RecordRequeueReason(metrics.FinalizersRemovalFromRemoteKyma, queue.UnexpectedRequeue)
		return r.requeueWithError(ctx, kyma, err)
	}

	logger.Info("removed remote finalizers")

	if err := r.kymaDeletionService.CleanupManifestCRs(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.CleanupManifestCrs, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}

	if err := r.kymaDeletionService.CleanupKyma(ctx, kyma); err != nil {
		err = fmt.Errorf("error while updating kyma during deletion: %w", err)
		r.Event.Warning(kyma, updateSpecError, err)
		r.Metrics.RecordRequeueReason(metrics.KymaDeletion, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}

	r.Metrics.RecordRequeueReason(metrics.KymaDeletion, queue.IntendedRequeue)
	return ctrl.Result{Requeue: true}, nil
}
