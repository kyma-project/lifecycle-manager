package manifest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/modulecr"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/status"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/resources"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/orphan"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	errManagerInErrorState            = errors.New("manager is in error state")
	errStateRequireUpdate             = errors.New("manifest state requires update")
	errResourceSyncDiffInSameOCILayer = errors.New("resource syncTarget diff detected but in " +
		"same oci layer, prevent sync resource to be deleted")
)

// SyncedOCIRefAnnotation is the annotation key on a Manifest CR that records
// the OCI ref last successfully synced to the SKR. The reconciler uses it to
// detect re-renders triggered by an OCI ref change.
const SyncedOCIRefAnnotation = "sync-oci-ref"

// stepResult is the outcome of a reconciliation step that may short-circuit
// the surrounding install/delete pipeline. A nil *stepResult means "continue
// to the next step"; a non-nil value carries the (ctrl.Result, error) tuple
// the controller-runtime caller must return immediately.
//
// We use a dedicated type rather than the previous (ctrl.Result, bool, error)
// triple, and we deliberately do NOT overload error, because several
// short-circuits in this controller are successful requeues (Err == nil).
type stepResult struct {
	Result ctrl.Result
	Err    error
}

// stopReconcile constructs a *stepResult signalling the caller should return
// (res, err) immediately. It exists so step bodies can compose directly with
// helpers that return (ctrl.Result, error): stopReconcile(r.finishReconcile(…)).
func stopReconcile(res ctrl.Result, err error) *stepResult {
	return &stepResult{Result: res, Err: err}
}

// SpecResolver resolves a Manifest CR to the location and identity of its
// installation layer (OCI ref, manifest YAML path).
type SpecResolver interface {
	GetSpec(ctx context.Context, manifest *v1beta2.Manifest) (*spec.Spec, error)
}

// ManifestAPIClient writes Manifest CRs and their statuses back to KCP.
type ManifestAPIClient interface {
	UpdateManifest(ctx context.Context, manifest *v1beta2.Manifest) error
	PatchStatusIfDiffExist(ctx context.Context, manifest *v1beta2.Manifest,
		previousStatus shared.Status,
	) error
	SsaSpec(ctx context.Context, obj client.Object) error
}

// OrphanDetectionService classifies a Manifest as orphaned when its owning
// Kyma CR no longer exists.
type OrphanDetectionService interface {
	DetectOrphanedManifest(ctx context.Context, manifest *v1beta2.Manifest) error
}

// SKRClientCache holds Kubernetes clients for SKR clusters keyed by the
// manifest's cache key.
type SKRClientCache interface {
	GetClient(key string) *skrclient.SKRClient
	AddClient(key string, client *skrclient.SKRClient)
	DeleteClient(key string)
}

// SKRClient resolves the Kubernetes client for the SKR if the SKRClientCache doesn't hold it already.
type SKRClient interface {
	ResolveClient(ctx context.Context, manifest *v1beta2.Manifest) (*skrclient.SKRClient, error)
}

// ResourceRenderService renders the target resources for a Manifest by parsing
// the manifest layer and applying any configured transforms. EvictCache drops
// the cached parse result for a Spec, forcing the next render to re-parse.
type ResourceRenderService interface {
	RenderTargetResources(ctx context.Context, skrClient skrclient.Client,
		manifest *v1beta2.Manifest, spec *spec.Spec) ([]client.Object, error)
	EvictCache(spec *spec.Spec)
}

// StateCheck reports the aggregated readiness state of a set of resources
// applied to the SKR.
type StateCheck interface {
	GetState(ctx context.Context, clnt client.Client, resources []client.Object) (shared.State, error)
}

// ManagedByLabelRemoval handles the cleanup of the managed-by label from
// resources when a Manifest CR transitions to unmanaged.
type ManagedByLabelRemoval interface {
	RemoveManagedByLabel(ctx context.Context, manifest *v1beta2.Manifest, skrClient client.Client) error
}

type Reconciler struct {
	requeueIntervals queue.RequeueIntervals
	rateLimiter      workqueue.TypedRateLimiter[ctrl.Request]
	kcpClient        client.Client
	renderService    ResourceRenderService
	customStateCheck StateCheck

	manifestMetrics            *metrics.ManifestMetrics
	mandatoryModuleMetrics     *metrics.MandatoryModulesMetrics
	specResolver               SpecResolver
	manifestClient             ManifestAPIClient
	managedLabelRemovalService ManagedByLabelRemoval
	orphanDetectionService     OrphanDetectionService
	skrClientCache             SKRClientCache
	skrClient                  SKRClient
}

func NewReconciler(requeueIntervals queue.RequeueIntervals,
	rateLimiter workqueue.TypedRateLimiter[ctrl.Request],
	manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestAPIClient ManifestAPIClient,
	orphanDetectionService OrphanDetectionService,
	specResolver SpecResolver,
	clientCache SKRClientCache,
	skrClient SKRClient,
	kcpClient client.Client,
	renderService ResourceRenderService,
	stateCheck StateCheck,
	managedLabelRemovalService ManagedByLabelRemoval,
) *Reconciler {
	return &Reconciler{
		requeueIntervals:           requeueIntervals,
		rateLimiter:                rateLimiter,
		kcpClient:                  kcpClient,
		renderService:              renderService,
		customStateCheck:           stateCheck,
		manifestMetrics:            manifestMetrics,
		mandatoryModuleMetrics:     mandatoryModulesMetrics,
		specResolver:               specResolver,
		manifestClient:             manifestAPIClient,
		managedLabelRemovalService: managedLabelRemovalService,
		orphanDetectionService:     orphanDetectionService,
		skrClientCache:             clientCache,
		skrClient:                  skrClient,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	startTime := time.Now()
	defer r.recordReconciliationDuration(startTime, req.Name)

	manifest := &v1beta2.Manifest{}
	if err := r.kcpClient.Get(ctx, req.NamespacedName, manifest); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Manifest %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{}, nil
		}
		r.manifestMetrics.RecordRequeueReason(metrics.ManifestRetrieval, queue.UnexpectedRequeue)
		return ctrl.Result{}, fmt.Errorf("manifestController: %w", err)
	}

	if manifest.SkipReconciliation() {
		logf.FromContext(ctx, "skip-label", shared.SkipReconcileLabel).
			V(internal.DebugLogLevel).Info("resource gets skipped because of label")
		return ctrl.Result{RequeueAfter: r.requeueIntervals.Success}, nil
	}

	if manifest.GetDeletionTimestamp().IsZero() {
		return r.install(ctx, req, manifest)
	}
	return r.delete(ctx, req, manifest)
}

// install handles the reconciliation in all conditions except when the manifest is being deleted.
// invariant: manifest.GetDeletionTimestamp().IsZero() == true
func (r *Reconciler) install(ctx context.Context, req ctrl.Request,
	manifest *v1beta2.Manifest,
) (ctrl.Result, error) {
	manifestStatus := manifest.GetStatus()

	skrClient, stop := r.prepareReconcile(ctx, req, manifest, manifestStatus)
	if stop != nil {
		return stop.Result, stop.Err
	}

	spec, stop := r.resolveAndTrackSpec(ctx, req, manifest, manifestStatus)
	if stop != nil {
		return stop.Result, stop.Err
	}

	target, current, err := r.renderResourcesForInstall(ctx, skrClient, manifest, spec)
	if err != nil {
		// The inject-data-from-kcp transform marks an unrecoverable misconfiguration
		// of the deployer module's secret wiring. Surface it as StateError so the
		// Kyma CR reflects the failure; the early return below already skips SSA.
		if errors.Is(err, render.ErrInjectDataFromKCP) {
			manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		}
		return r.finishReconcile(ctx, manifest, metrics.ManifestRenderResources, manifestStatus, err)
	}

	if stop := r.pruneAndSync(ctx, req, skrClient, manifest, manifestStatus,
		current, target, spec); stop != nil {
		return stop.Result, stop.Err
	}

	if err := r.syncDefaultModuleCR(ctx, skrClient, manifest); err != nil {
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncState, manifestStatus, err)
	}

	if err := finalizer.EnsureCRFinalizer(ctx, r.kcpClient, manifest); err != nil {
		if errors.Is(err, finalizer.ErrRequeueRequired) {
			r.manifestMetrics.RecordRequeueReason(metrics.ManifestSyncResourcesEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{RequeueAfter: r.rateLimiter.When(req)}, nil
		}
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncState, manifestStatus, err)
	}

	if err := r.updateManifestStateAfterSync(ctx, skrClient, manifest, target); err != nil {
		logf.FromContext(ctx).Error(err, "failed to sync manifest state")
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncState, manifestStatus, err)
	}

	// This situation happens when manifest get new installation layer to update resources,
	// we need to make sure all updates successfully before we can update synced oci ref
	if requireUpdateSyncedOCIRefAnnotation(manifest, spec.OCIRef) {
		updateSyncedOCIRefAnnotation(manifest, spec.OCIRef)
		return r.updateManifest(ctx, req, manifest, metrics.ManifestUpdateSyncedOCIRef)
	}

	return r.finishReconcile(ctx, manifest, metrics.ManifestReconcileFinished, manifestStatus, nil)
}

// prepareReconcile runs the preparatory steps at the start of the install
// pipeline — status init, mandatory-module metric, SKR client resolution,
// unmanaged short-circuit, orphan detection, and finalizer SSA — before the
// spec is resolved or resources are rendered. A non-nil *stepResult signals
// the caller must return immediately.
func (r *Reconciler) prepareReconcile(ctx context.Context, req ctrl.Request,
	manifest *v1beta2.Manifest, manifestStatus shared.Status,
) (skrclient.Client, *stepResult) {
	if err := status.Initialize(manifest); err != nil {
		return nil, stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestInit, manifestStatus, err))
	}

	recordMandatoryModuleState(manifest, r)

	skrClient, err := r.getTargetClient(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		return nil, stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestClientInit, manifestStatus, err))
	}

	if manifest.IsUnmanaged() {
		return nil, stopReconcile(r.handleUnmanagedManifest(ctx, req, skrClient, manifest))
	}

	if err := r.orphanDetectionService.DetectOrphanedManifest(ctx, manifest); err != nil {
		if errors.Is(err, orphan.ErrOrphanedManifest) {
			previousStatus := manifest.GetStatus()
			manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
			return nil, stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestOrphaned, previousStatus, err))
		}
		return nil, stopReconcile(ctrl.Result{}, fmt.Errorf("manifestController: %w", err))
	}

	if finalizer.FinalizersUpdateRequired(manifest) {
		return nil, stopReconcile(r.ssaSpec(ctx, req, manifest, metrics.ManifestAddFinalizer))
	}

	return skrClient, nil
}

// resolveAndTrackSpec resolves the install spec and persists the initial
// synced-OCI-ref annotation. A non-nil *stepResult signals the caller must
// return immediately.
func (r *Reconciler) resolveAndTrackSpec(ctx context.Context, req ctrl.Request,
	manifest *v1beta2.Manifest, manifestStatus shared.Status,
) (*spec.Spec, *stepResult) {
	spec, err := r.specResolver.GetSpec(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		return nil, stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestParseSpec, manifestStatus, err))
	}

	if notContainsSyncedOCIRefAnnotation(manifest) {
		updateSyncedOCIRefAnnotation(manifest, spec.OCIRef)
		return nil, stopReconcile(r.updateManifest(ctx, req, manifest, metrics.ManifestInitSyncedOCIRef))
	}

	return spec, nil
}

// pruneAndSync runs the diff prune followed by an SSA sync of the target
// resources. A non-nil *stepResult signals the caller must return immediately
// (either because pruning is still in progress or because pruning/sync errored).
func (r *Reconciler) pruneAndSync(ctx context.Context, req ctrl.Request, skrClient skrclient.Client,
	manifest *v1beta2.Manifest, manifestStatus shared.Status,
	current ResourceList, target []client.Object, spec *spec.Spec,
) *stepResult {
	if err := r.pruneDiff(ctx, skrClient, manifest, current, target, spec); errors.Is(err,
		resources.ErrDeletionNotFinished) {
		r.manifestMetrics.RecordRequeueReason(metrics.ManifestPruneDiffNotFinished, queue.IntendedRequeue)
		return stopReconcile(ctrl.Result{RequeueAfter: r.rateLimiter.When(req)}, nil)
	} else if err != nil {
		return stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestPruneDiff, manifestStatus, err))
	}

	if err := skrresources.SyncResources(ctx, skrClient, manifest, target); err != nil {
		return stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestSyncResources, manifestStatus, err))
	}

	return nil
}

// delete handles the reconciliation when the manifest is being deleted.
// invariant: manifest.GetDeletionTimestamp().IsZero() == false
func (r *Reconciler) delete(ctx context.Context, req ctrl.Request,
	manifest *v1beta2.Manifest,
) (ctrl.Result, error) {
	manifestStatus := manifest.GetStatus()

	skrClient, stop := r.prepareDeleteReconcile(ctx, req, manifest, manifestStatus)
	if stop != nil {
		return stop.Result, stop.Err
	}

	spec, stop := r.resolveAndTrackSpecForDelete(ctx, req, manifest, manifestStatus)
	if stop != nil {
		return stop.Result, stop.Err
	}

	target, current, err := r.renderResourcesForDelete(ctx, skrClient, manifest, spec)
	if err != nil {
		return r.finishReconcile(ctx, manifest, metrics.ManifestRenderResources, manifestStatus, err)
	}

	if manifestUnderDeletingButNoSyncedResources(manifest, current) {
		r.evictSKRClientCache(ctx, manifest)
	}

	if err := r.pruneDiff(ctx, skrClient, manifest, current, target, spec); errors.Is(err,
		resources.ErrDeletionNotFinished) {
		r.manifestMetrics.RecordRequeueReason(metrics.ManifestPruneDiffNotFinished, queue.IntendedRequeue)
		return ctrl.Result{RequeueAfter: r.rateLimiter.When(req)}, nil
	} else if err != nil {
		return r.finishReconcile(ctx, manifest, metrics.ManifestPruneDiff, manifestStatus, err)
	}

	if err := modulecr.NewClient(skrClient).RemoveDefaultModuleCR(ctx, r.kcpClient, manifest); err != nil {
		if errors.Is(err, finalizer.ErrRequeueRequired) {
			r.manifestMetrics.RecordRequeueReason(metrics.ManifestPreDeleteEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{RequeueAfter: r.rateLimiter.When(req)}, nil
		}
		return r.finishReconcile(ctx, manifest, metrics.ManifestPreDelete, manifestStatus, err)
	}

	if err := skrresources.SyncResources(ctx, skrClient, manifest, target); err != nil {
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncResources, manifestStatus, err)
	}

	if err := r.updateDeletingState(manifest); err != nil {
		logf.FromContext(ctx).Error(err, "failed to sync manifest state")
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncState, manifestStatus, err)
	}

	// This situation happens when manifest get new installation layer to update resources,
	// we need to make sure all updates successfully before we can update synced oci ref
	if requireUpdateSyncedOCIRefAnnotation(manifest, spec.OCIRef) {
		updateSyncedOCIRefAnnotation(manifest, spec.OCIRef)
		return r.updateManifest(ctx, req, manifest, metrics.ManifestUpdateSyncedOCIRef)
	}

	return r.cleanupManifest(ctx, req, manifest, manifestStatus, metrics.ManifestReconcileFinished, nil)
}

// prepareDeleteReconcile runs the preparatory steps at the start of the
// delete pipeline — status init, mandatory-module metric, SKR client
// resolution (with access-secret cleanup short-circuit), and unmanaged
// cleanup — before the spec is resolved or resources are rendered. A non-nil
// *stepResult signals the caller must return immediately.
func (r *Reconciler) prepareDeleteReconcile(ctx context.Context, req ctrl.Request,
	manifest *v1beta2.Manifest, manifestStatus shared.Status,
) (skrclient.Client, *stepResult) {
	if err := status.Initialize(manifest); err != nil {
		return nil, stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestInit, manifestStatus, err))
	}

	recordMandatoryModuleState(manifest, r)

	skrClient, err := r.getTargetClient(ctx, manifest)
	if err != nil {
		if errors.Is(err, accessmanager.ErrAccessSecretNotFound) {
			return nil, stopReconcile(r.cleanupManifest(ctx, req, manifest, manifestStatus,
				metrics.ManifestClientInit, err))
		}

		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		return nil, stopReconcile(r.finishReconcile(ctx, manifest, metrics.ManifestClientInit, manifestStatus, err))
	}

	if manifest.IsUnmanaged() {
		return nil, stopReconcile(r.cleanupManifest(ctx, req, manifest, manifestStatus,
			metrics.ManifestUnmanagedUpdate, nil))
	}

	return skrClient, nil
}

// resolveAndTrackSpecForDelete is the delete-path equivalent of
// resolveAndTrackSpec — error path routes through cleanupManifest because
// we're tearing down.
func (r *Reconciler) resolveAndTrackSpecForDelete(ctx context.Context, req ctrl.Request,
	manifest *v1beta2.Manifest, manifestStatus shared.Status,
) (*spec.Spec, *stepResult) {
	spec, err := r.specResolver.GetSpec(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		return nil, stopReconcile(r.cleanupManifest(ctx, req, manifest, manifestStatus,
			metrics.ManifestParseSpec, err))
	}

	if notContainsSyncedOCIRefAnnotation(manifest) {
		updateSyncedOCIRefAnnotation(manifest, spec.OCIRef)
		return nil, stopReconcile(r.updateManifest(ctx, req, manifest, metrics.ManifestInitSyncedOCIRef))
	}

	return spec, nil
}

// Normally after all resources have been deleted, the manifest should be cleared as well.
// But when this happens, it might be the connection to the SKR is lost which prevents the manifest deletion,
// so the next step is try to evict the cache and hope the next reconciliation can determine
// if the skr kubeconfig secret is deleted or not. If the secret is deleted, then the manifest will be deleted as well.
func manifestUnderDeletingButNoSyncedResources(manifest *v1beta2.Manifest, current ResourceList) bool {
	return !manifest.GetDeletionTimestamp().IsZero() && len(current) == 0
}

func recordMandatoryModuleState(manifest *v1beta2.Manifest, r *Reconciler) {
	if manifest.IsMandatoryModule() {
		state := manifest.GetStatus().State
		kymaName := manifest.GetLabels()[shared.KymaName]
		moduleName := manifest.GetLabels()[shared.ModuleName]
		r.mandatoryModuleMetrics.RecordMandatoryModuleState(kymaName, moduleName, state)
	}
}

func (r *Reconciler) handleLabelsRemovalFinalizer(ctx context.Context, req ctrl.Request, skrClient client.Client,
	manifest *v1beta2.Manifest,
) (ctrl.Result, error) {
	err := r.managedLabelRemovalService.RemoveManagedByLabel(ctx, manifest, skrClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.manifestMetrics.RecordRequeueReason(metrics.ManifestResourcesLabelRemoval, queue.IntendedRequeue)
	return ctrl.Result{RequeueAfter: r.rateLimiter.When(req)}, nil
}

func (r *Reconciler) cleanupManifest(
	ctx context.Context, req ctrl.Request, manifest *v1beta2.Manifest, manifestStatus shared.Status,
	requeueReason metrics.ManifestRequeueReason, originalErr error,
) (ctrl.Result, error) {
	err := r.cleanupMetrics(manifest)
	if err != nil {
		return ctrl.Result{}, err
	}
	var finalizerRemoved bool
	if errors.Is(originalErr, accessmanager.ErrAccessSecretNotFound) || manifest.IsUnmanaged() {
		finalizerRemoved = finalizer.RemoveAllFinalizers(manifest)
	} else {
		finalizerRemoved = finalizer.RemoveRequiredFinalizers(manifest)
	}
	if finalizerRemoved {
		return r.updateManifest(ctx, req, manifest, requeueReason)
	}
	if manifest.GetStatus().State != shared.StateWarning {
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateDeleting).
			WithOperation(fmt.Sprintf("waiting as other finalizers are present: %s", manifest.GetFinalizers())))
	}
	return r.finishReconcile(ctx, manifest, requeueReason, manifestStatus, originalErr)
}

func (r *Reconciler) cleanupMetrics(manifest *v1beta2.Manifest) error {
	kymaName, err := manifest.GetKymaName()
	if err != nil {
		return fmt.Errorf("failed to get kyma name: %w", err)
	}
	moduleName, err := manifest.GetModuleName()
	if err != nil {
		return fmt.Errorf("failed to get module name: %w", err)
	}
	r.manifestMetrics.CleanupMetrics(manifest.GetName())

	if manifest.IsMandatoryModule() {
		r.mandatoryModuleMetrics.CleanupMetrics(kymaName, moduleName)
	}
	return nil
}

func (r *Reconciler) evictSKRClientCache(ctx context.Context, manifest *v1beta2.Manifest) {
	clientsCacheKey, found := manifest.GenerateCacheKey()
	if found {
		logf.FromContext(ctx).Info("Invalidating manifest-controller client cache entry for key: " + fmt.Sprintf("%#v",
			clientsCacheKey))
		r.skrClientCache.DeleteClient(clientsCacheKey)
	}
}

func (r *Reconciler) renderResourcesForInstall(ctx context.Context, skrClient skrclient.Client,
	manifest *v1beta2.Manifest, spec *spec.Spec,
) ([]client.Object, ResourceList, error) {
	manifestStatus := manifest.GetStatus()
	current := ResourceList(manifestStatus.Synced)

	target, err := r.renderService.RenderTargetResources(ctx, skrClient, manifest, spec)
	if err != nil {
		return nil, nil, err
	}

	status.SetResourcesConditionTrue(manifest)
	return target, current, nil
}

func (r *Reconciler) renderResourcesForDelete(ctx context.Context, skrClient skrclient.Client,
	manifest *v1beta2.Manifest, spec *spec.Spec,
) ([]client.Object, ResourceList, error) {
	manifestStatus := manifest.GetStatus()
	current := ResourceList(manifestStatus.Synced)

	allModuleCRsDeleted, err := ensureModuleCRsAllDeleted(ctx, skrClient, manifest)
	switch {
	case allModuleCRsDeleted:
		return []client.Object{}, current, nil
	case errors.Is(err, modulecr.ErrWaitingForModuleCRsDeletion):
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateDeleting).
			WithOperation("waiting for module crs deletion"))
		return nil, nil, err
	case err != nil:
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return nil, nil, err
	}

	// we're here only if allModuleCRsDeleted == false and err == nil.
	target, err := r.renderService.RenderTargetResources(ctx, skrClient, manifest, spec)
	if err != nil {
		return nil, nil, err
	}

	status.SetResourcesConditionTrue(manifest)
	return target, current, nil
}

func ensureModuleCRsAllDeleted(ctx context.Context, skrClient skrclient.Client, manifest *v1beta2.Manifest) (
	bool,
	error,
) {
	if err := modulecr.NewClient(skrClient).CheckModuleCRsDeletion(ctx, manifest); err != nil {
		return false, err
	}

	return modulecr.NewClient(skrClient).CheckDefaultCRDeletion(ctx, manifest)
}

func (r *Reconciler) handleUnmanagedManifest(ctx context.Context, req ctrl.Request,
	skrClient client.Client, manifest *v1beta2.Manifest,
) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(manifest, finalizer.LabelRemovalFinalizer) {
		return r.handleLabelsRemovalFinalizer(ctx, req, skrClient, manifest)
	}
	if err := r.kcpClient.Delete(ctx, manifest); err != nil {
		return ctrl.Result{}, fmt.Errorf("manifestController: %w", err)
	}
	return ctrl.Result{RequeueAfter: r.requeueIntervals.Success}, nil
}

func (r *Reconciler) syncDefaultModuleCR(ctx context.Context, skrClient skrclient.Client,
	manifest *v1beta2.Manifest,
) error {
	manifestStatus := manifest.GetStatus()
	if manifest.ShouldCreateDefaultModuleCR() && !status.IsModuleCRInstallConditionTrue(manifestStatus) {
		if err := modulecr.NewClient(skrClient).SyncDefaultModuleCR(ctx, manifest); err != nil {
			manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
			return err
		}
		status.SetModuleCRInstallConditionTrue(manifest)
	}
	return nil
}

func (r *Reconciler) updateManifestStateAfterSync(ctx context.Context, skrClient skrclient.Client,
	manifest *v1beta2.Manifest, target []client.Object,
) error {
	manifestStatus := manifest.GetStatus()
	managerState, err := r.checkManagerState(ctx, skrClient, target)
	if err != nil {
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return err
	}
	if status.RequireManifestStateUpdateAfterSyncResource(manifest, managerState) {
		return fmt.Errorf("%w: from %s to %s", errStateRequireUpdate, manifestStatus.State, managerState)
	}
	return nil
}

func (r *Reconciler) updateDeletingState(manifest *v1beta2.Manifest) error {
	manifestStatus := manifest.GetStatus()
	if status.RequireManifestStateUpdateAfterSyncResource(manifest, shared.StateDeleting) {
		return fmt.Errorf("%w: from %s to %s", errStateRequireUpdate, manifestStatus.State, shared.StateDeleting)
	}
	return nil
}

func (r *Reconciler) checkManagerState(ctx context.Context, clnt skrclient.Client,
	target []client.Object,
) (shared.State, error) {
	managerReadyCheck := r.customStateCheck
	managerState, err := managerReadyCheck.GetState(ctx, clnt, target)
	if err != nil {
		return shared.StateError, err
	}
	if managerState == shared.StateError {
		return shared.StateError, errManagerInErrorState
	}
	return managerState, nil
}

func (r *Reconciler) pruneDiff(ctx context.Context, clnt skrclient.Client, manifest *v1beta2.Manifest,
	current ResourceList, target []client.Object, spec *spec.Spec,
) error {
	diff := pruneResource(current.Difference(target), "Namespace", shared.DefaultRemoteNamespace)
	if len(diff) == 0 {
		return nil
	}
	if manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff, manifest, spec) {
		// This case should not happen normally, but if happens, it means the resources read from cache is incomplete,
		// and we should prevent diff resources to be deleted.
		// Meanwhile, evict cache to hope newly created resources back to normal.
		manifest.SetStatus(
			manifest.GetStatus().
				WithState(shared.StateWarning).
				WithOperation(errResourceSyncDiffInSameOCILayer.Error()),
		)
		r.renderService.EvictCache(spec)
		return errResourceSyncDiffInSameOCILayer
	}

	err := resources.NewConcurrentCleanup(clnt, manifest).DeleteDiffResources(ctx, diff)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithErr(err))
	}
	return err
}

func manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff ResourceList, manifest *v1beta2.Manifest,
	spec *spec.Spec,
) bool {
	return len(diff) > 0 && ociRefNotChanged(manifest, spec.OCIRef) && manifest.GetDeletionTimestamp().IsZero()
}

func ociRefNotChanged(manifest *v1beta2.Manifest, ref string) bool {
	syncedOCIRef, found := manifest.GetAnnotations()[SyncedOCIRefAnnotation]
	return found && syncedOCIRef == ref
}

func requireUpdateSyncedOCIRefAnnotation(manifest *v1beta2.Manifest, ref string) bool {
	syncedOCIRef, found := manifest.GetAnnotations()[SyncedOCIRefAnnotation]
	if found && syncedOCIRef != ref {
		return true
	}
	return false
}

func notContainsSyncedOCIRefAnnotation(manifest *v1beta2.Manifest) bool {
	_, found := manifest.GetAnnotations()[SyncedOCIRefAnnotation]
	return !found
}

func updateSyncedOCIRefAnnotation(manifest *v1beta2.Manifest, ref string) {
	annotations := manifest.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[SyncedOCIRefAnnotation] = ref
	manifest.SetAnnotations(annotations)
}

func pruneResource(diff ResourceList, resourceType string, resourceName string) ResourceList {
	for index, res := range diff {
		if res.Kind == resourceType && res.Name == resourceName {
			return append(diff[:index], diff[index+1:]...)
		}
	}
	return diff
}

func (r *Reconciler) getTargetClient(ctx context.Context, manifest *v1beta2.Manifest) (skrclient.Client, error) {
	var err error
	var clnt *skrclient.SKRClient

	clientsCacheKey, found := manifest.GenerateCacheKey()
	if found {
		clnt = r.skrClientCache.GetClient(clientsCacheKey)
	}

	if clnt == nil {
		clnt, err = r.skrClient.ResolveClient(ctx, manifest)
		if err != nil {
			return nil, err
		}
		r.skrClientCache.AddClient(clientsCacheKey, clnt)
	}

	return clnt, nil
}

func (r *Reconciler) finishReconcile(ctx context.Context, manifest *v1beta2.Manifest,
	requeueReason metrics.ManifestRequeueReason, previousStatus shared.Status, originalErr error,
) (ctrl.Result, error) {
	if err := r.manifestClient.PatchStatusIfDiffExist(ctx, manifest, previousStatus); err != nil {
		logf.FromContext(ctx).Error(err, "failed to patch manifest status when finishing reconciliation",
			"previousStatus", previousStatus,
			"newStatus", manifest.GetStatus(),
		)
		return ctrl.Result{}, err
	}
	switch {
	case util.IsConnectionRelatedError(originalErr):
		logf.FromContext(ctx).Error(originalErr, "connection related error during reconciliation",
			"requeueReason", requeueReason)
		r.evictSKRClientCache(ctx, manifest)
		r.manifestMetrics.RecordRequeueReason(metrics.ManifestUnauthorized, queue.UnexpectedRequeue)
		return ctrl.Result{}, originalErr
	case originalErr != nil:
		logf.FromContext(ctx).Error(originalErr, "error during reconciliation",
			"requeueReason", requeueReason)
		r.manifestMetrics.RecordRequeueReason(requeueReason, queue.UnexpectedRequeue)
		return ctrl.Result{}, originalErr
	default:
		r.manifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
		requeueAfter := queue.DetermineRequeueInterval(manifest.GetStatus().State, r.requeueIntervals)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
}

func (r *Reconciler) ssaSpec(ctx context.Context, req ctrl.Request, manifest *v1beta2.Manifest,
	requeueReason metrics.ManifestRequeueReason,
) (ctrl.Result, error) {
	r.manifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
	if err := r.manifestClient.SsaSpec(ctx, manifest); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.rateLimiter.When(req)}, nil
}

func (r *Reconciler) updateManifest(ctx context.Context, req ctrl.Request, manifest *v1beta2.Manifest,
	requeueReason metrics.ManifestRequeueReason,
) (ctrl.Result, error) {
	r.manifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)

	if err := r.manifestClient.UpdateManifest(ctx, manifest); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: r.rateLimiter.When(req)}, nil
}

func (r *Reconciler) recordReconciliationDuration(startTime time.Time, name string) {
	duration := time.Since(startTime)
	if duration >= 1*time.Minute {
		r.manifestMetrics.RecordManifestDuration(name, duration)
	} else {
		r.manifestMetrics.RemoveManifestDuration(name)
	}
}
