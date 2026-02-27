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
	"os"
	"time"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

var (
	ErrManifestsStillExist = errors.New("manifests still exist")
	ErrInvalidKymaSpec     = errors.New("invalid kyma spec")
	ErrKymaInErrorState    = errors.New("kyma in error state")
)

const (
	metricsError      event.Reason = "MetricsError"
	updateSpecError   event.Reason = "UpdateSpecError"
	updateStatusError event.Reason = "UpdateStatusError"
	patchStatusError  event.Reason = "PatchStatus"
)

type DeletionMetricWriter interface {
	Write(res result.Result)
}

type DeletionEventRecorder interface {
	Record(ctx context.Context, obj machineryruntime.Object, res result.Result)
}

type DeletionService interface {
	Delete(ctx context.Context, kyma *v1beta2.Kyma) result.Result
}

type LookupService interface {
	ByRuntimeID(ctx context.Context, runtimeID string) (*v1beta2.Kyma, error)
}

type SKRWebhookManager interface {
	Reconcile(ctx context.Context, kyma *v1beta2.Kyma) error
	Remove(ctx context.Context, kyma *v1beta2.Kyma) error
	RemoveSkrCertificate(ctx context.Context, kymaName string) error
}

type ModuleStatusHandler interface {
	UpdateModuleStatuses(ctx context.Context, kyma *v1beta2.Kyma, modules modulecommon.Modules) error
}

type SkrSyncService interface {
	SyncCrds(ctx context.Context, kyma *v1beta2.Kyma) (bool, error)
	SyncImagePullSecret(ctx context.Context, kyma types.NamespacedName) error
}

// ReconcilerConfig holds configuration values for the Kyma Reconciler.
// Usually read from flags or environment variables.
type ReconcilerConfig struct {
	RemoteSyncNamespace    string
	OCIRegistryHost        string
	SkrImagePullSecretName string
}

type Reconciler struct {
	client.Client
	event.Event
	queue.RequeueIntervals

	RateLimiter workqueue.TypedRateLimiter[ctrl.Request]

	Config               ReconcilerConfig
	SkrContextFactory    remote.SkrContextProvider
	DescriptorProvider   *provider.CachedDescriptorProvider
	SkrSyncService       SkrSyncService
	ModulesStatusHandler ModuleStatusHandler
	SKRWebhookManager    SKRWebhookManager

	Metrics        *metrics.KymaMetrics
	RemoteCatalog  *remote.RemoteCatalog
	TemplateLookup *templatelookup.TemplateLookup

	DeletionMetrics DeletionMetricWriter
	DeletionEvents  DeletionEventRecorder
	DeletionService DeletionService
	LookupService   LookupService
}

// Reconcile reconciles Kyma resources.
//
// See https://github.com/kyma-project/lifecycle-manager/issues/2943.
//
//nolint:funlen // disable for kyma controller until we remove legacy deletion with above issue
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Kyma reconciliation started")

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
		return ctrl.Result{}, fmt.Errorf("KymaController: %w", err)
	}

	if err := r.UpdateModuleTemplatesIfNeeded(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("KymaController: %w", err)
	}

	status.InitConditions(kyma, r.WatcherEnabled(), r.SkrImagePullSecretSyncEnabled())

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info("skipping reconciliation for Kyma: " + kyma.Name)
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	err := r.SkrContextFactory.Init(ctx, kyma.GetNamespacedName())
	if !kyma.DeletionTimestamp.IsZero() && errors.Is(err, accessmanager.ErrAccessSecretNotFound) {
		return r.handleDeletedSkr(ctx, req, kyma)
	}

	skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.SyncContextRetrieval, queue.UnexpectedRequeue)
		setModuleStatusesToError(kyma, err.Error())
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, err)
	}

	if !kyma.DeletionTimestamp.IsZero() {
		envValue, isDefined := os.LookupEnv("ENABLE_LEGACY_KYMA_DELETION")
		useLegacyKymaDeletion := isDefined && envValue == "true"
		if !useLegacyKymaDeletion {
			return r.processDeletion(ctx, kyma)
		}
	}

	err = skrContext.CreateKymaNamespace(ctx)
	if apierrors.IsUnauthorized(err) {
		r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
		logger.Info("connection refused, assuming connection is invalid and resetting cache-entry for kyma")
		r.Metrics.RecordRequeueReason(metrics.KymaUnauthorized, queue.UnexpectedRequeue)
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, err)
	}
	if err != nil {
		r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
		r.Metrics.RecordRequeueReason(metrics.SyncContextRetrieval, queue.UnexpectedRequeue)
		setModuleStatusesToError(kyma, util.NestedErrorMessage(err))
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, err)
	}

	return r.reconcile(ctx, req, kyma)
}

// ValidateDefaultChannel validates the Kyma spec.
func (r *Reconciler) ValidateDefaultChannel(kyma *v1beta2.Kyma) error {
	if shared.NoneChannel.Equals(kyma.Spec.Channel) {
		return fmt.Errorf("%w: value \"none\" is not allowed in spec.channel", ErrInvalidKymaSpec)
	}
	return nil
}

func (r *Reconciler) DeleteNoLongerExistingModules(ctx context.Context, kyma *v1beta2.Kyma) error {
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

func (r *Reconciler) UpdateMetrics(ctx context.Context, kyma *v1beta2.Kyma) {
	if err := r.Metrics.UpdateAll(kyma); err != nil {
		if metrics.IsMissingMetricsAnnotationOrLabel(err) {
			r.Event.Warning(kyma, metricsError, err)
		}
		logf.FromContext(ctx).V(log.DebugLevel).Info(fmt.Sprintf("error occurred while updating all metrics: %s", err))
	}
}

func (r *Reconciler) WatcherEnabled() bool {
	return r.SKRWebhookManager != nil
}

func (r *Reconciler) SkrImagePullSecretSyncEnabled() bool {
	return r.Config.SkrImagePullSecretName != ""
}

func (r *Reconciler) GetModuleTemplateList(ctx context.Context) (*v1beta2.ModuleTemplateList, error) {
	moduleTemplateList := &v1beta2.ModuleTemplateList{}
	if err := r.List(ctx, moduleTemplateList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("could not aggregate module templates for module catalog sync: %w", err)
	}

	return moduleTemplateList, nil
}

func (r *Reconciler) UpdateModuleTemplatesIfNeeded(ctx context.Context) error {
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

func (r *Reconciler) processDeletion(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	res := r.DeletionService.Delete(ctx, kyma)

	if util.IsConnectionRelatedError(res.Err) {
		r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
	}

	r.DeletionMetrics.Write(res)
	r.DeletionEvents.Record(ctx, kyma, res)

	switch res.UseCase {
	case usecase.SetKcpKymaStateDeleting,
		usecase.SetSkrKymaStateDeleting,
		usecase.DeleteSkrKyma,
		usecase.DeleteWatcherCertificateSetup,
		usecase.DeleteSkrWebhookResources,
		usecase.DeleteSkrModuleTemplateCrd,
		usecase.DeleteSkrModuleReleaseMetaCrd,
		usecase.DeleteSkrKymaCrd,
		usecase.DeleteManifests,
		usecase.DeleteMetrics:
		// error takes precedence over the RequeueAfter
		// res.Err != nil => requeue rate limited
		// res.Err == nil => requeue after
		return ctrl.Result{RequeueAfter: 1 * time.Second}, res.Err
	case usecase.DropKymaFinalizer:
		// finalizers removed, no need to requeue if there is no error
	}
	return ctrl.Result{}, res.Err
}

func (r *Reconciler) handleDeletedSkr(ctx context.Context, req ctrl.Request, kyma *v1beta2.Kyma) (ctrl.Result, error) {
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
	return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
}

//nolint:funlen // disable for kyma controller until split is done into provisioning and deprovisioning controllers
func (r *Reconciler) reconcile(ctx context.Context, req ctrl.Request, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != shared.StateDeleting {
		if err := r.deleteRemoteKyma(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.RemoteKymaDeletion, queue.UnexpectedRequeue)
			return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, err)
		}
		if err := r.updateStatus(ctx, kyma, shared.StateDeleting, "waiting for modules to be deleted"); err != nil {
			r.Metrics.RecordRequeueReason(metrics.StatusUpdateToDeleting, queue.UnexpectedRequeue)
			return ctrl.Result{}, r.updateStatusWithError(ctx, kyma,
				fmt.Errorf("could not update kyma status after triggering deletion: %w", err))
		}
		r.Metrics.RecordRequeueReason(metrics.StatusUpdateToDeleting, queue.IntendedRequeue)
		return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
	}

	if needsUpdate := kyma.EnsureLabelsAndFinalizers(); needsUpdate {
		if err := r.Update(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.LabelsAndFinalizersUpdate, queue.UnexpectedRequeue)
			return ctrl.Result{}, r.updateStatusWithError(ctx, kyma,
				fmt.Errorf("failed to update kyma after finalizer check: %w", err))
		}
		r.Metrics.RecordRequeueReason(metrics.LabelsAndFinalizersUpdate, queue.IntendedRequeue)
		return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
	}

	updateRequired, err := r.SkrSyncService.SyncCrds(ctx, kyma)
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.CrdsSync, queue.UnexpectedRequeue)
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, fmt.Errorf("could not sync CRDs: %w", err))
	}
	if updateRequired {
		if err := r.Update(ctx, kyma); err != nil {
			r.Metrics.RecordRequeueReason(metrics.CrdAnnotationsUpdate, queue.UnexpectedRequeue)
			return ctrl.Result{}, r.updateStatusWithError(ctx, kyma,
				fmt.Errorf("could not update kyma annotations: %w", err))
		}
		r.Metrics.RecordRequeueReason(metrics.CrdAnnotationsUpdate, queue.IntendedRequeue)
		return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
	}

	if r.SkrImagePullSecretSyncEnabled() {
		if err := r.SkrSyncService.SyncImagePullSecret(ctx, kyma.GetNamespacedName()); err != nil {
			r.Metrics.RecordRequeueReason(metrics.ImagePullSecretSync, queue.UnexpectedRequeue)
			kyma.UpdateCondition(v1beta2.ConditionTypeSKRImagePullSecretSync, apimetav1.ConditionFalse)
			return ctrl.Result{}, r.updateStatusWithError(ctx, kyma,
				fmt.Errorf("could not sync image pull secret: %w", err))
		}
		kyma.UpdateCondition(v1beta2.ConditionTypeSKRImagePullSecretSync, apimetav1.ConditionTrue)
	}

	// update the control-plane kyma with the changes to the spec of the remote Kyma
	if err = r.replaceSpecFromRemote(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.SpecReplacementFromRemote, queue.UnexpectedRequeue)
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, fmt.Errorf("could not replace control plane kyma spec"+
			" with remote kyma spec: %w", err))
	}

	res, err := r.processKymaState(ctx, req, kyma)
	if err != nil {
		r.Metrics.RecordRequeueReason(metrics.ProcessingKymaState, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}

	if err := r.syncStatusToRemote(ctx, kyma); err != nil {
		r.Metrics.RecordRequeueReason(metrics.StatusSyncToRemote, queue.UnexpectedRequeue)
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma,
			fmt.Errorf("could not synchronize remote kyma status: %w", err))
	}

	if kyma.Status.State == shared.StateError {
		// Requeue with a new Error in case of Kyma error state, to enable rate limiting for that error.
		return ctrl.Result{}, ErrKymaInErrorState
	}

	return res, nil
}

func (r *Reconciler) deleteRemoteKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
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

func (r *Reconciler) fetchRemoteKyma(ctx context.Context, kcpKyma *v1beta2.Kyma) (*v1beta2.Kyma, error) {
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
func (r *Reconciler) syncStatusToRemote(ctx context.Context, kcpKyma *v1beta2.Kyma) error {
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
func (r *Reconciler) replaceSpecFromRemote(ctx context.Context, controlPlaneKyma *v1beta2.Kyma) error {
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

func (r *Reconciler) processKymaState(ctx context.Context, req ctrl.Request, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	switch kyma.Status.State {
	case "":
		return r.handleInitialState(ctx, req, kyma)
	case shared.StateProcessing:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateDeleting:
		return r.handleDeletingState(ctx, req, kyma)
	case shared.StateError:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateReady, shared.StateWarning:
		return r.handleProcessingState(ctx, kyma)
	case shared.StateUnmanaged:
		return ctrl.Result{}, nil // no requeue of invalid state
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) handleInitialState(
	ctx context.Context, req ctrl.Request, kyma *v1beta2.Kyma,
) (ctrl.Result, error) {
	if err := r.updateStatus(ctx, kyma, shared.StateProcessing, "started processing"); err != nil {
		r.Metrics.RecordRequeueReason(metrics.InitialStateHandling, queue.UnexpectedRequeue)
		return ctrl.Result{}, err
	}
	r.Metrics.RecordRequeueReason(metrics.InitialStateHandling, queue.IntendedRequeue)
	return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
}

func (r *Reconciler) handleProcessingState(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
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
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, err)
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
	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

func checkSKRWebhookReadiness(ctx context.Context, skrClient *remote.SkrContext, kyma *v1beta2.Kyma) error {
	err := watcher.AssertDeploymentReady(ctx, skrClient)
	if err != nil {
		kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, apimetav1.ConditionFalse)
		if errors.Is(err, watcher.ErrSkrWebhookDeploymentInBackoff) {
			return err
		}
		return nil
	}
	kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, apimetav1.ConditionTrue)
	return nil
}

func (r *Reconciler) handleDeletingState(
	ctx context.Context, req ctrl.Request, kyma *v1beta2.Kyma,
) (ctrl.Result, error) {
	if r.WatcherEnabled() {
		if err := r.SKRWebhookManager.Remove(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.RemoteCatalog.Delete(ctx, kyma.GetNamespacedName()); err != nil {
		err = fmt.Errorf("failed to delete remote module catalog: %w", err)
		r.Metrics.RecordRequeueReason(metrics.RemoteModuleCatalogDeletion, queue.UnexpectedRequeue)
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, err)
	}
	skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get skrContext: %w", err)
	}

	r.SkrContextFactory.InvalidateCache(kyma.GetNamespacedName())
	if err = skrContext.RemoveFinalizersFromKyma(ctx); client.IgnoreNotFound(err) != nil {
		r.Metrics.RecordRequeueReason(metrics.FinalizersRemovalFromRemoteKyma, queue.UnexpectedRequeue)
		return ctrl.Result{}, r.updateStatusWithError(ctx, kyma, err)
	}

	logger := logf.FromContext(ctx).V(log.InfoLevel)
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
	return ctrl.Result{RequeueAfter: r.RateLimiter.When(req)}, nil
}

func (r *Reconciler) cleanupMetrics(kymaName string) {
	r.Metrics.CleanupMetrics(kymaName)
}

func (r *Reconciler) cleanupManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) error {
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

func (r *Reconciler) deleteManifests(ctx context.Context, manifests []v1beta2.Manifest) error {
	for i := range manifests {
		if err := r.Delete(ctx, &manifests[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("error while trying to delete manifest: %w", err)
		}
	}
	return nil
}

func (r *Reconciler) getRelatedManifestCRs(ctx context.Context, kyma *v1beta2.Kyma) ([]v1beta2.Manifest, error) {
	manifestList := &v1beta2.ManifestList{}
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kyma.Name})
	if err := r.List(ctx, manifestList,
		&client.ListOptions{LabelSelector: labelSelector}); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("failed to get related manifests, %w", err)
	}

	return manifestList.Items, nil
}

func (r *Reconciler) relatedManifestCRsAreDeleted(manifests []v1beta2.Manifest) bool {
	return len(manifests) == 0
}

func (r *Reconciler) removeAllFinalizers(kyma *v1beta2.Kyma) {
	for _, finalizer := range kyma.Finalizers {
		controllerutil.RemoveFinalizer(kyma, finalizer)
	}
}

func (r *Reconciler) updateKyma(ctx context.Context, kyma *v1beta2.Kyma) error {
	if err := r.Update(ctx, kyma); err != nil {
		err = fmt.Errorf("error while updating kyma during deletion: %w", err)
		r.Event.Warning(kyma, updateSpecError, err)
		return err
	}

	return nil
}

func (r *Reconciler) reconcileManifests(ctx context.Context, kyma *v1beta2.Kyma) error {
	templates := r.TemplateLookup.GetRegularTemplates(ctx, kyma)
	prsr := parser.NewParser(r.Client, r.DescriptorProvider, r.Config.RemoteSyncNamespace, r.Config.OCIRegistryHost)
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

func (r *Reconciler) updateStatus(ctx context.Context, kyma *v1beta2.Kyma,
	state shared.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		r.Event.Warning(kyma, patchStatusError, err)
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *Reconciler) updateStatusWithError(ctx context.Context, kyma *v1beta2.Kyma, err error) error {
	if statusErr := status.Helper(r).UpdateStatusForExistingModules(
		ctx, kyma, shared.StateError, err.Error()); statusErr != nil {
		r.Event.Warning(kyma, updateStatusError, statusErr)
		// Return both errors - status update failure wraps the original error
		return fmt.Errorf("error while updating status to %s: %w", shared.StateError, errors.Join(err, statusErr))
	}
	return err
}

func (r *Reconciler) deleteManifest(ctx context.Context, trackedManifest *v1beta2.TrackingObject) error {
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

func (r *Reconciler) deleteOrphanedCertificate(ctx context.Context, kymaName string) error {
	if r.WatcherEnabled() {
		if err := r.SKRWebhookManager.RemoveSkrCertificate(ctx, kymaName); err != nil {
			return err
		}
	}
	return nil
}

func needUpdateForMandatoryModuleLabel(moduleTemplate v1beta2.ModuleTemplate) bool {
	if moduleTemplate.Labels == nil {
		moduleTemplate.Labels = make(map[string]string)
	}

	if moduleTemplate.Spec.Mandatory {
		if moduleTemplate.Labels[shared.IsMandatoryModule] == shared.EnableLabelValue {
			return false
		}

		moduleTemplate.Labels[shared.IsMandatoryModule] = shared.EnableLabelValue
		return true
	}

	if !moduleTemplate.Spec.Mandatory {
		if moduleTemplate.Labels[shared.IsMandatoryModule] == shared.EnableLabelValue {
			delete(moduleTemplate.Labels, shared.IsMandatoryModule)
			return true
		}
	}

	return false
}

func setModuleStatusesToError(kyma *v1beta2.Kyma, message string) {
	moduleStatuses := kyma.Status.Modules
	for i := range moduleStatuses {
		moduleStatuses[i].State = shared.StateError
		if message != "" {
			moduleStatuses[i].Message = message
		}
	}
}
