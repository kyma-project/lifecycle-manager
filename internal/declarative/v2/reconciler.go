package v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/labelsremoval"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/modulecr"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/status"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrResourceSyncDiffInSameOCILayer = errors.New("resource syncTarget diff detected but in " +
	"same oci layer, prevent sync resource to be deleted")

const (
	namespaceNotBeRemoved  = "kyma-system"
	SyncedOCIRefAnnotation = "sync-oci-ref"
)

func NewFromManager(mgr manager.Manager, requeueIntervals queue.RequeueIntervals, metrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics, moduleMetrics *metrics.ModuleMetrics,
	manifestAPIClient ManifestAPIClient, specResolver SpecResolver, options ...Option,
) *Reconciler {
	reconciler := &Reconciler{}
	reconciler.ManifestMetrics = metrics
	reconciler.MandatoryModuleMetrics = mandatoryModulesMetrics
	reconciler.ModuleMetrics = moduleMetrics
	reconciler.RequeueIntervals = requeueIntervals
	reconciler.specResolver = specResolver
	reconciler.manifestClient = manifestAPIClient
	reconciler.managedLabelRemovalService = labelsremoval.NewManagedLabelRemovalService(manifestAPIClient)
	reconciler.Options = DefaultOptions().Apply(WithManager(mgr)).Apply(options...)
	return reconciler
}

type ManagedLabelRemoval interface {
	RemoveManagedLabel(ctx context.Context,
		manifest *v1beta2.Manifest, skrClient client.Client, defaultCR *unstructured.Unstructured,
	) error
}

type ManifestAPIClient interface {
	UpdateManifest(ctx context.Context, manifest *v1beta2.Manifest) error
	PatchStatusIfDiffExist(ctx context.Context, manifest *v1beta2.Manifest,
		previousStatus shared.Status) error
	SsaSpec(ctx context.Context, obj client.Object) error
}

type Reconciler struct {
	queue.RequeueIntervals
	*Options
	ManifestMetrics            *metrics.ManifestMetrics
	MandatoryModuleMetrics     *metrics.MandatoryModulesMetrics
	ModuleMetrics              *metrics.ModuleMetrics
	specResolver               SpecResolver
	manifestClient             ManifestAPIClient
	managedLabelRemovalService ManagedLabelRemoval
}

//nolint:funlen,cyclop,gocognit // Declarative pkg will be removed soon
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	defer r.recordReconciliationDuration(startTime, req.Name)

	manifest := &v1beta2.Manifest{}
	if err := r.Get(ctx, req.NamespacedName, manifest); err != nil {
		if util.IsNotFound(err) {
			logf.FromContext(ctx).Info(req.NamespacedName.String() + " got deleted!")
			return ctrl.Result{}, nil
		}
		r.ManifestMetrics.RecordRequeueReason(metrics.ManifestRetrieval, queue.UnexpectedRequeue)
		return ctrl.Result{}, fmt.Errorf("manifestController: %w", err)
	}
	manifestStatus := manifest.GetStatus()

	if manifest.SkipReconciliation() {
		logf.FromContext(ctx, "skip-label", shared.SkipReconcileLabel).
			V(internal.DebugLogLevel).Info("resource gets skipped because of label")
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	if err := status.Initialize(manifest); err != nil {
		return r.finishReconcile(ctx, manifest, metrics.ManifestInit, manifestStatus, err)
	}

	recordMandatoryModuleState(manifest, r)

	skrClient, err := r.getTargetClient(ctx, manifest)
	if err != nil {
		if !manifest.GetDeletionTimestamp().IsZero() && errors.Is(err, common.ErrAccessSecretNotFound) {
			return r.cleanupManifest(ctx, manifest, manifestStatus, metrics.ManifestClientInit, err)
		}

		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		return r.finishReconcile(ctx, manifest, metrics.ManifestClientInit, manifestStatus, err)
	}

	if manifest.IsUnmanaged() {
		if !manifest.GetDeletionTimestamp().IsZero() {
			return r.cleanupManifest(ctx, manifest, manifestStatus, metrics.ManifestUnmanagedUpdate, nil)
		}

		if controllerutil.ContainsFinalizer(manifest, labelsremoval.LabelRemovalFinalizer) {
			return r.handleLabelsRemovalFinalizer(ctx, skrClient, manifest)
		}

		if err := r.Delete(ctx, manifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("manifestController: %w", err)
		}
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	if manifest.GetDeletionTimestamp().IsZero() {
		if finalizer.FinalizersUpdateRequired(manifest) {
			return r.ssaSpec(ctx, manifest, metrics.ManifestAddFinalizer)
		}
	}

	spec, err := r.specResolver.GetSpec(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		if !manifest.GetDeletionTimestamp().IsZero() {
			return r.cleanupManifest(ctx, manifest, manifestStatus, metrics.ManifestParseSpec, err)
		}
		return r.finishReconcile(ctx, manifest, metrics.ManifestParseSpec, manifestStatus, err)
	}

	if notContainsSyncedOCIRefAnnotation(manifest) {
		updateSyncedOCIRefAnnotation(manifest, spec.OCIRef)
		return r.updateManifest(ctx, manifest, metrics.ManifestInitSyncedOCIRef)
	}

	target, current, err := r.renderResources(ctx, skrClient, manifest, spec)
	if err != nil {
		if util.IsConnectionRelatedError(err) {
			r.invalidateClientCache(ctx, manifest)
			return r.finishReconcile(ctx, manifest, metrics.ManifestUnauthorized, manifestStatus, err)
		}

		return r.finishReconcile(ctx, manifest, metrics.ManifestRenderResources, manifestStatus, err)
	}

	if err := r.pruneDiff(ctx, skrClient, manifest, current, target, spec); errors.Is(err,
		resources.ErrDeletionNotFinished) {
		r.ManifestMetrics.RecordRequeueReason(metrics.ManifestPruneDiffNotFinished, queue.IntendedRequeue)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		return r.finishReconcile(ctx, manifest, metrics.ManifestPruneDiff, manifestStatus, err)
	}

	if !manifest.GetDeletionTimestamp().IsZero() {
		if err := modulecr.NewClient(skrClient).RemoveModuleCR(ctx, r.Client, manifest); err != nil {
			if errors.Is(err, finalizer.ErrRequeueRequired) {
				r.ManifestMetrics.RecordRequeueReason(metrics.ManifestPreDeleteEnqueueRequired, queue.IntendedRequeue)
				return ctrl.Result{Requeue: true}, nil
			}
			return r.finishReconcile(ctx, manifest, metrics.ManifestPreDelete, manifestStatus, err)
		}
	}

	if err := skrresources.SyncResources(ctx, skrClient, manifest, target); err != nil {
		if errors.Is(err, skrresources.ErrClientUnauthorized) {
			r.invalidateClientCache(ctx, manifest)
		}
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncResources, manifestStatus, err)
	}

	if err := r.syncManifestState(ctx, skrClient, manifest, target); err != nil {
		if errors.Is(err, finalizer.ErrRequeueRequired) {
			r.ManifestMetrics.RecordRequeueReason(metrics.ManifestSyncResourcesEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		logf.FromContext(ctx).Error(err, "failed to sync manifest state")
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncState, manifestStatus, err)
	}
	// This situation happens when manifest get new installation layer to update resources,
	// we need to make sure all updates successfully before we can update synced oci ref
	if requireUpdateSyncedOCIRefAnnotation(manifest, spec.OCIRef) {
		updateSyncedOCIRefAnnotation(manifest, spec.OCIRef)
		return r.updateManifest(ctx, manifest, metrics.ManifestUpdateSyncedOCIRef)
	}

	if !manifest.GetDeletionTimestamp().IsZero() {
		return r.cleanupManifest(ctx, manifest, manifestStatus, metrics.ManifestReconcileFinished, nil)
	}

	return r.finishReconcile(ctx, manifest, metrics.ManifestReconcileFinished, manifestStatus, nil)
}

func recordMandatoryModuleState(manifest *v1beta2.Manifest, r *Reconciler) {
	if manifest.IsMandatoryModule() {
		state := manifest.GetStatus().State
		kymaName := manifest.GetLabels()[shared.KymaName]
		moduleName := manifest.GetLabels()[shared.ModuleName]
		r.MandatoryModuleMetrics.RecordMandatoryModuleState(kymaName, moduleName, state)
	}
}

func (r *Reconciler) handleLabelsRemovalFinalizer(ctx context.Context, skrClient client.Client,
	manifest *v1beta2.Manifest,
) (ctrl.Result, error) {
	defaultCR, err := modulecr.NewClient(skrClient).GetCR(ctx, manifest)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.managedLabelRemovalService.RemoveManagedLabel(ctx, manifest, skrClient,
		defaultCR); err != nil {
		return ctrl.Result{}, err
	}

	r.ManifestMetrics.RecordRequeueReason(metrics.ManifestResourcesLabelRemoval, queue.IntendedRequeue)
	return ctrl.Result{Requeue: true}, nil
}

func (r *Reconciler) cleanupManifest(ctx context.Context, manifest *v1beta2.Manifest, manifestStatus shared.Status,
	requeueReason metrics.ManifestRequeueReason, originalErr error,
) (ctrl.Result, error) {
	err := r.cleanupMetrics(manifest)
	if err != nil {
		return ctrl.Result{}, err
	}
	var finalizerRemoved bool
	if errors.Is(originalErr, common.ErrAccessSecretNotFound) || manifest.IsUnmanaged() {
		finalizerRemoved = finalizer.RemoveAllFinalizers(manifest)
	} else {
		finalizerRemoved = finalizer.RemoveRequiredFinalizers(manifest)
	}
	if finalizerRemoved {
		return r.updateManifest(ctx, manifest, requeueReason)
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
	r.ManifestMetrics.CleanupMetrics(manifest.GetName())

	if manifest.IsMandatoryModule() {
		r.MandatoryModuleMetrics.CleanupMetrics(kymaName, moduleName)
	}
	return nil
}

func (r *Reconciler) invalidateClientCache(ctx context.Context, manifest *v1beta2.Manifest) {
	if r.ClientCacheKeyFn != nil {
		clientsCacheKey, ok := r.ClientCacheKeyFn(ctx, manifest)
		if ok {
			logf.FromContext(ctx).Info("Invalidating manifest-controller client cache entry for key: " + fmt.Sprintf("%#v",
				clientsCacheKey))
			r.ClientCache.DeleteClient(clientsCacheKey)
		}
	}
}

func (r *Reconciler) renderResources(ctx context.Context, skrClient Client, manifest *v1beta2.Manifest,
	spec *Spec,
) ([]*resource.Info, []*resource.Info, error) {
	manifestStatus := manifest.GetStatus()

	var err error
	var target, current ResourceList

	converter := skrresources.NewResourceToInfoConverter(skrresources.ResourceInfoConverter(skrClient),
		apimetav1.NamespaceDefault)

	if target, err = r.renderTargetResources(ctx, skrClient, converter, manifest, spec); err != nil {
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return nil, nil, err
	}

	current, err = converter.ResourcesToInfos(manifestStatus.Synced)
	if err != nil {
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return nil, nil, err
	}
	status.ConfirmResourcesCondition(manifest)
	return target, current, nil
}

func (r *Reconciler) syncManifestState(ctx context.Context, skrClient Client, manifest *v1beta2.Manifest,
	target []*resource.Info,
) error {
	manifestStatus := manifest.GetStatus()

	moduleCRState, err := modulecr.NewClient(skrClient).SyncModuleCR(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return err
	}

	if err := finalizer.EnsureCRFinalizer(ctx, r.Client, manifest); err != nil {
		return err
	}
	if !manifest.GetDeletionTimestamp().IsZero() {
		if moduleCRState == shared.StateWarning {
			if err := r.RecordModuleCRWarningCondition(manifest); err != nil {
				return err
			}
			status.ConfirmModuleCRCondition(manifest)
		}
		return status.SetManifestState(manifest, shared.StateDeleting)
	}

	managerState, err := r.checkManagerState(ctx, skrClient, target)
	if err != nil {
		manifest.SetStatus(manifestStatus.WithState(shared.StateError).WithErr(err))
		return err
	}
	return status.SetManifestState(manifest, managerState)
}

func (r *Reconciler) RecordModuleCRWarningCondition(manifest *v1beta2.Manifest) error {
	kymaName, err := manifest.GetKymaName()
	if err != nil {
		return fmt.Errorf("failed to get kyma name: %w", err)
	}
	moduleName, err := manifest.GetModuleName()
	if err != nil {
		return fmt.Errorf("failed to get module name: %w", err)
	}
	r.ModuleMetrics.SetModuleCRWarningCondition(kymaName, moduleName)
	return nil
}

func (r *Reconciler) RemoveModuleCRWarningCondition(manifest *v1beta2.Manifest) error {
	kymaName, err := manifest.GetKymaName()
	if err != nil {
		return fmt.Errorf("failed to get kyma name: %w", err)
	}
	moduleName, err := manifest.GetModuleName()
	if err != nil {
		return fmt.Errorf("failed to get module name: %w", err)
	}
	r.ModuleMetrics.RemoveModuleCRWarningCondition(kymaName, moduleName)
	return nil
}

func (r *Reconciler) checkManagerState(ctx context.Context, clnt Client, target []*resource.Info) (shared.State,
	error,
) {
	managerReadyCheck := r.CustomStateCheck
	managerState, err := managerReadyCheck.GetState(ctx, clnt, target)
	if err != nil {
		return shared.StateError, err
	}

	return managerState, nil
}

func (r *Reconciler) renderTargetResources(ctx context.Context, skrClient client.Client,
	converter skrresources.ResourceToInfoConverter, manifest *v1beta2.Manifest, spec *Spec,
) ([]*resource.Info, error) {
	if !manifest.GetDeletionTimestamp().IsZero() {
		deleted, err := modulecr.NewClient(skrClient).CheckCRDeletion(ctx, manifest)
		if err != nil {
			return nil, err
		}
		if deleted {
			return ResourceList{}, nil
		}
	}

	targetResources, err := r.ManifestParser.Parse(spec)
	if err != nil {
		return nil, err
	}

	for _, transform := range r.PostRenderTransforms {
		if err := transform(ctx, manifest, targetResources.Items); err != nil {
			return nil, err
		}
	}

	target, err := converter.UnstructuredToInfos(targetResources.Items)
	if err != nil {
		return nil, err
	}

	return target, nil
}

func (r *Reconciler) pruneDiff(ctx context.Context, clnt Client, manifest *v1beta2.Manifest,
	current, target []*resource.Info, spec *Spec,
) error {
	diff, err := pruneResource(ResourceList(current).Difference(target), "Namespace", namespaceNotBeRemoved)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithErr(err))
		return err
	}
	if len(diff) == 0 {
		return nil
	}
	if manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff, manifest, spec) {
		// This case should not happen normally, but if happens, it means the resources read from cache is incomplete,
		// and we should prevent diff resources to be deleted.
		// Meanwhile, evict cache to hope newly created resources back to normal.
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateWarning).WithOperation(ErrResourceSyncDiffInSameOCILayer.Error()))
		r.ManifestParser.EvictCache(spec)
		return ErrResourceSyncDiffInSameOCILayer
	}

	err = resources.NewConcurrentCleanup(clnt, manifest).DeleteDiffResources(ctx, diff)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithErr(err))
	}

	return err
}

func manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff []*resource.Info, manifest *v1beta2.Manifest,
	spec *Spec,
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

func pruneResource(diff []*resource.Info, resourceType string, resourceName string) ([]*resource.Info, error) {
	for index, info := range diff {
		obj, ok := info.Object.(client.Object)
		if !ok {
			return diff, common.ErrTypeAssert
		}
		if obj.GetObjectKind().GroupVersionKind().Kind == resourceType && obj.GetName() == resourceName {
			return append(diff[:index], diff[index+1:]...), nil
		}
	}

	return diff, nil
}

func (r *Reconciler) getTargetClient(ctx context.Context, manifest *v1beta2.Manifest) (Client, error) {
	var err error
	var clnt Client
	if r.ClientCacheKeyFn == nil {
		return r.configClient(ctx, manifest)
	}

	clientsCacheKey, found := r.ClientCacheKeyFn(ctx, manifest)
	if found {
		clnt = r.GetClient(clientsCacheKey)
	}

	if clnt == nil {
		clnt, err = r.configClient(ctx, manifest)
		if err != nil {
			return nil, err
		}
		r.AddClient(clientsCacheKey, clnt)
	}

	return clnt, nil
}

func (r *Reconciler) configClient(ctx context.Context, manifest *v1beta2.Manifest) (Client, error) {
	var err error

	cluster := &ClusterInfo{
		Config: r.Config,
		Client: r.Client,
	}

	if r.TargetCluster != nil {
		cluster, err = r.TargetCluster(ctx, manifest)
		if err != nil {
			return nil, err
		}
	}

	clnt, err := NewSingletonClients(cluster)
	if err != nil {
		return nil, err
	}

	return clnt, nil
}

func (r *Reconciler) finishReconcile(ctx context.Context, manifest *v1beta2.Manifest,
	requeueReason metrics.ManifestRequeueReason, previousStatus shared.Status, originalErr error,
) (ctrl.Result, error) {
	if err := r.manifestClient.PatchStatusIfDiffExist(ctx, manifest, previousStatus); err != nil {
		return ctrl.Result{}, err
	}
	if originalErr != nil {
		r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.UnexpectedRequeue)
		return ctrl.Result{}, originalErr
	}
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
	requeueAfter := queue.DetermineRequeueInterval(manifest.GetStatus().State, r.RequeueIntervals)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *Reconciler) ssaSpec(ctx context.Context, manifest *v1beta2.Manifest,
	requeueReason metrics.ManifestRequeueReason,
) (ctrl.Result, error) {
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
	if err := r.manifestClient.SsaSpec(ctx, manifest); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *Reconciler) updateManifest(ctx context.Context, manifest *v1beta2.Manifest,
	requeueReason metrics.ManifestRequeueReason,
) (ctrl.Result, error) {
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)

	if err := r.manifestClient.UpdateManifest(ctx, manifest); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *Reconciler) recordReconciliationDuration(startTime time.Time, name string) {
	duration := time.Since(startTime)
	if duration >= 1*time.Minute {
		r.ManifestMetrics.RecordManifestDuration(name, duration)
	} else {
		r.ManifestMetrics.RemoveManifestDuration(name)
	}
}
