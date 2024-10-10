package v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/labelsremoval"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrWarningResourceSyncStateDiff   = errors.New("resource syncTarget state diff detected")
	ErrResourceSyncDiffInSameOCILayer = errors.New("resource syncTarget diff detected but in " +
		"same oci layer, prevent sync resource to be deleted")
	ErrInstallationConditionRequiresUpdate = errors.New("installation condition needs an update")
	ErrObjectHasEmptyState                 = errors.New("object has an empty state")
	ErrRequeueRequired                     = errors.New("requeue required")
	ErrAccessSecretNotFound                = errors.New("access secret not found")
)

const (
	namespaceNotBeRemoved                            = "kyma-system"
	CustomResourceManagerFinalizer                   = "resource.kyma-project.io/finalizer"
	SyncedOCIRefAnnotation                           = "sync-oci-ref"
	defaultFinalizer                                 = "declarative.kyma-project.io/finalizer"
	defaultFieldOwner              client.FieldOwner = "declarative.kyma-project.io/applier"
)

func NewFromManager(mgr manager.Manager,
	requeueIntervals queue.RequeueIntervals,
	metrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestClient manifestclient.ManifestClient,
	specResolver SpecResolver,
	options ...Option,
) *Reconciler {
	reconciler := &Reconciler{}
	reconciler.ManifestMetrics = metrics
	reconciler.MandatoryModuleMetrics = mandatoryModulesMetrics
	reconciler.RequeueIntervals = requeueIntervals
	reconciler.specResolver = specResolver
	reconciler.manifestClient = manifestClient
	reconciler.managedLabelRemovalService = labelsremoval.NewManagedLabelRemovalService(manifestClient)
	reconciler.Options = DefaultOptions().Apply(WithManager(mgr)).Apply(options...)
	return reconciler
}

type Reconciler struct {
	queue.RequeueIntervals
	*Options
	ManifestMetrics            *metrics.ManifestMetrics
	MandatoryModuleMetrics     *metrics.MandatoryModulesMetrics
	specResolver               SpecResolver
	manifestClient             manifestclient.ManifestClient
	managedLabelRemovalService *labelsremoval.ManagedLabelRemovalService
}

const waitingForResourcesMsg = "waiting for resources to become ready"

type ConditionType string

const (
	ConditionTypeResources    ConditionType = "Resources"
	ConditionTypeInstallation ConditionType = "Installation"
)

type ConditionReason string

const (
	ConditionReasonResourcesAreAvailable ConditionReason = "ResourcesAvailable"
	ConditionReasonReady                 ConditionReason = "Ready"
)

func newInstallationCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeInstallation),
		Reason:             string(ConditionReasonReady),
		Status:             apimetav1.ConditionFalse,
		Message:            "installation is ready and resources can be used",
		ObservedGeneration: manifest.GetGeneration(),
	}
}

func newResourcesCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeResources),
		Reason:             string(ConditionReasonResourcesAreAvailable),
		Status:             apimetav1.ConditionFalse,
		Message:            "resources are parsed and ready for use",
		ObservedGeneration: manifest.GetGeneration(),
	}
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

	if err := r.initialize(manifest); err != nil {
		return r.finishReconcile(ctx, manifest, metrics.ManifestInit, manifestStatus, err)
	}

	if manifest.IsMandatoryModule() {
		state := manifest.GetStatus().State
		kymaName := manifest.GetLabels()[shared.KymaName]
		moduleName := manifest.GetLabels()[shared.ModuleName]
		r.MandatoryModuleMetrics.RecordMandatoryModuleState(kymaName, moduleName, state)
	}

	skrClient, err := r.getTargetClient(ctx, manifest)
	if err != nil {
		if !manifest.GetDeletionTimestamp().IsZero() && errors.Is(err, ErrAccessSecretNotFound) {
			return r.cleanupManifest(ctx, req, manifest, manifestStatus, metrics.ManifestClientInit,
				err)
		}

		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		return r.finishReconcile(ctx, manifest, metrics.ManifestClientInit, manifestStatus, err)
	}

	if manifest.IsUnmanaged() {
		if !manifest.GetDeletionTimestamp().IsZero() {
			return r.cleanupManifest(ctx, req, manifest, manifestStatus, metrics.ManifestUnmanagedUpdate, nil)
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
		partialMeta := r.partialObjectMetadata(manifest)
		defaultFinalizerAdded := controllerutil.AddFinalizer(partialMeta, defaultFinalizer)
		labelRemovalFinalizerAdded := controllerutil.AddFinalizer(partialMeta, labelsremoval.LabelRemovalFinalizer)
		if defaultFinalizerAdded || labelRemovalFinalizerAdded {
			return r.ssaSpec(ctx, partialMeta, metrics.ManifestAddFinalizer)
		}
	}

	spec, err := r.specResolver.GetSpec(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateError).WithErr(err))
		if !manifest.GetDeletionTimestamp().IsZero() {
			return r.cleanupManifest(ctx, req, manifest, manifestStatus, metrics.ManifestParseSpec, err)
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

	if err := r.removeModuleCR(ctx, skrClient, manifest); err != nil {
		if errors.Is(err, ErrRequeueRequired) {
			r.ManifestMetrics.RecordRequeueReason(metrics.ManifestPreDeleteEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		return r.finishReconcile(ctx, manifest, metrics.ManifestPreDelete, manifestStatus, err)
	}

	if err = r.syncResources(ctx, skrClient, manifest, target); err != nil {
		if errors.Is(err, ErrRequeueRequired) {
			r.ManifestMetrics.RecordRequeueReason(metrics.ManifestSyncResourcesEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		if errors.Is(err, ErrClientUnauthorized) {
			r.invalidateClientCache(ctx, manifest)
		}
		return r.finishReconcile(ctx, manifest, metrics.ManifestSyncResources, manifestStatus, err)
	}

	// This situation happens when manifest get new installation layer to update resources,
	// we need to make sure all updates successfully before we can update synced oci ref
	if requireUpdateSyncedOCIRefAnnotation(manifest, spec.OCIRef) {
		updateSyncedOCIRefAnnotation(manifest, spec.OCIRef)
		return r.updateManifest(ctx, manifest, metrics.ManifestUpdateSyncedOCIRef)
	}

	if !manifest.GetDeletionTimestamp().IsZero() {
		return r.cleanupManifest(ctx, req, manifest, manifestStatus, metrics.ManifestReconcileFinished, nil)
	}

	return r.finishReconcile(ctx, manifest, metrics.ManifestReconcileFinished, manifestStatus, nil)
}

func (r *Reconciler) handleLabelsRemovalFinalizer(ctx context.Context, skrClient client.Client,
	manifest *v1beta2.Manifest,
) (ctrl.Result, error) {
	defaultCR, err := getCR(ctx, skrClient, manifest)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.managedLabelRemovalService.HandleLabelsRemovalFinalizerForUnmanagedModule(ctx, manifest, skrClient,
		defaultCR); err != nil {
		return ctrl.Result{}, err
	}

	r.ManifestMetrics.RecordRequeueReason(metrics.ManifestResourcesLabelRemoval, queue.IntendedRequeue)
	return ctrl.Result{Requeue: true}, nil
}

func (r *Reconciler) cleanupManifest(ctx context.Context, req ctrl.Request, manifest *v1beta2.Manifest,
	manifestStatus shared.Status, requeueReason metrics.ManifestRequeueReason, originalErr error,
) (ctrl.Result, error) {
	r.ManifestMetrics.RemoveManifestDuration(req.Name)
	r.cleanUpMandatoryModuleMetrics(manifest)
	finalizersToRemove := []string{defaultFinalizer, labelsremoval.LabelRemovalFinalizer}
	if errors.Is(originalErr, ErrAccessSecretNotFound) || manifest.IsUnmanaged() {
		finalizersToRemove = manifest.GetFinalizers()
	}
	if removeFinalizers(manifest, finalizersToRemove) {
		return r.updateManifest(ctx, manifest, requeueReason)
	}
	if manifest.GetStatus().State != shared.StateWarning {
		manifest.SetStatus(manifest.GetStatus().WithState(shared.StateDeleting).
			WithOperation(fmt.Sprintf("waiting as other finalizers are present: %s", manifest.GetFinalizers())))
	}
	return r.finishReconcile(ctx, manifest, requeueReason, manifestStatus, originalErr)
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

func removeFinalizers(manifest *v1beta2.Manifest, finalizersToRemove []string) bool {
	finalizerRemoved := false
	for _, f := range finalizersToRemove {
		if controllerutil.RemoveFinalizer(manifest, f) {
			finalizerRemoved = true
		}
	}

	return finalizerRemoved
}

func (r *Reconciler) partialObjectMetadata(manifest *v1beta2.Manifest) *apimetav1.PartialObjectMetadata {
	objMeta := &apimetav1.PartialObjectMetadata{}
	objMeta.SetName(manifest.GetName())
	objMeta.SetNamespace(manifest.GetNamespace())
	objMeta.SetGroupVersionKind(manifest.GetObjectKind().GroupVersionKind())
	objMeta.SetFinalizers(manifest.GetFinalizers())
	return objMeta
}

func (r *Reconciler) initialize(manifest *v1beta2.Manifest) error {
	status := manifest.GetStatus()

	for _, condition := range []apimetav1.Condition{
		newResourcesCondition(manifest),
		newInstallationCondition(manifest),
	} {
		if meta.FindStatusCondition(status.Conditions, condition.Type) == nil {
			meta.SetStatusCondition(&status.Conditions, condition)
		}
	}

	if status.Synced == nil {
		status.Synced = []shared.Resource{}
	}

	if status.State == "" {
		manifest.SetStatus(status.WithState(shared.StateProcessing).WithErr(ErrObjectHasEmptyState))
		return ErrObjectHasEmptyState
	}

	manifest.SetStatus(status)

	return nil
}

func (r *Reconciler) renderResources(ctx context.Context, skrClient Client, manifest *v1beta2.Manifest,
	spec *Spec,
) ([]*resource.Info, []*resource.Info, error) {
	resourceCondition := newResourcesCondition(manifest)
	status := manifest.GetStatus()

	var err error
	var target, current ResourceList

	converter := NewResourceToInfoConverter(ResourceInfoConverter(skrClient), apimetav1.NamespaceDefault)

	if target, err = r.renderTargetResources(ctx, skrClient, converter, manifest, spec); err != nil {
		manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, nil, err
	}

	current, err = converter.ResourcesToInfos(status.Synced)
	if err != nil {
		manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, nil, err
	}

	if !meta.IsStatusConditionTrue(status.Conditions, resourceCondition.Type) {
		resourceCondition.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, resourceCondition)
		manifest.SetStatus(status.WithOperation(resourceCondition.Message))
	}

	return target, current, nil
}

func (r *Reconciler) syncResources(ctx context.Context, clnt Client, manifest *v1beta2.Manifest,
	target []*resource.Info,
) error {
	status := manifest.GetStatus()

	if err := ConcurrentSSA(clnt, defaultFieldOwner).Run(ctx, target); err != nil {
		manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return err
	}

	oldSynced := status.Synced
	newSynced := NewInfoToResourceConverter().InfosToResources(target)
	status.Synced = newSynced

	if hasDiff(oldSynced, newSynced) {
		if manifest.GetDeletionTimestamp().IsZero() {
			manifest.SetStatus(status.WithState(shared.StateProcessing).WithOperation(ErrWarningResourceSyncStateDiff.Error()))
		} else if status.State != shared.StateWarning {
			manifest.SetStatus(status.WithState(shared.StateDeleting).WithOperation(ErrWarningResourceSyncStateDiff.Error()))
		}
		return ErrWarningResourceSyncStateDiff
	}

	for i := range r.PostRuns {
		if err := r.PostRuns[i](ctx, clnt, r.Client, manifest); err != nil {
			manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
			return err
		}
	}

	if !manifest.GetDeletionTimestamp().IsZero() {
		return r.setManifestState(manifest, shared.StateDeleting)
	}

	managerState, err := r.checkManagerState(ctx, clnt, target)
	if err != nil {
		manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return err
	}
	return r.setManifestState(manifest, managerState)
}

func hasDiff(oldResources []shared.Resource, newResources []shared.Resource) bool {
	if len(oldResources) != len(newResources) {
		return true
	}
	countMap := map[string]bool{}
	for _, item := range oldResources {
		countMap[item.ID()] = true
	}
	for _, item := range newResources {
		if countMap[item.ID()] {
			countMap[item.ID()] = false
		}
	}
	for _, exists := range countMap {
		if exists {
			return true
		}
	}
	return false
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

func (r *Reconciler) setManifestState(manifest *v1beta2.Manifest, newState shared.State) error {
	status := manifest.GetStatus()

	if newState == shared.StateProcessing {
		manifest.SetStatus(status.WithState(shared.StateProcessing).WithOperation(waitingForResourcesMsg))
		return ErrInstallationConditionRequiresUpdate
	}

	installationCondition := newInstallationCondition(manifest)
	if newState != status.State || !meta.IsStatusConditionTrue(status.Conditions, installationCondition.Type) {
		installationCondition.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, installationCondition)

		manifest.SetStatus(status.WithState(newState).WithOperation(installationCondition.Message))
		return ErrInstallationConditionRequiresUpdate
	}

	return nil
}

func (r *Reconciler) removeModuleCR(ctx context.Context, clnt Client, manifest *v1beta2.Manifest) error {
	if !manifest.GetDeletionTimestamp().IsZero() {
		for _, preDelete := range r.PreDeletes {
			if err := preDelete(ctx, clnt, r.Client, manifest); err != nil {
				// we do not set a status here since it will be deleting if timestamp is set.
				manifest.SetStatus(manifest.GetStatus().WithErr(err))
				return err
			}
		}
	}
	return nil
}

func (r *Reconciler) renderTargetResources(ctx context.Context, skrClient client.Client,
	converter ResourceToInfoConverter, manifest *v1beta2.Manifest, spec *Spec,
) ([]*resource.Info, error) {
	if !manifest.GetDeletionTimestamp().IsZero() {
		deleted, err := checkCRDeletion(ctx, skrClient, manifest)
		if err != nil {
			return nil, err
		}
		if deleted {
			return ResourceList{}, nil
		}
	}

	status := manifest.GetStatus()

	targetResources, err := r.ManifestParser.Parse(spec)
	if err != nil {
		manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, err
	}

	for _, transform := range r.PostRenderTransforms {
		if err := transform(ctx, manifest, targetResources.Items); err != nil {
			manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
			return nil, err
		}
	}

	target, err := converter.UnstructuredToInfos(targetResources.Items)
	if err != nil {
		manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, err
	}

	return target, nil
}

func checkCRDeletion(ctx context.Context, skrClient client.Client, manifestCR *v1beta2.Manifest) (bool,
	error,
) {
	if manifestCR.Spec.Resource == nil {
		return true, nil
	}

	resourceCR, err := getCR(ctx, skrClient, manifestCR)
	if err != nil {
		if util.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}

	return resourceCR == nil, nil
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

func (r *Reconciler) ssaSpec(ctx context.Context, obj client.Object,
	requeueReason metrics.ManifestRequeueReason,
) (ctrl.Result, error) {
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
	if err := r.manifestClient.SsaSpec(ctx, obj); err != nil {
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

func (r *Reconciler) cleanUpMandatoryModuleMetrics(manifest *v1beta2.Manifest) {
	if manifest.IsMandatoryModule() {
		kymaName := manifest.GetLabels()[shared.KymaName]
		moduleName := manifest.GetLabels()[shared.ModuleName]
		r.MandatoryModuleMetrics.CleanupMetrics(kymaName, moduleName)
	}
}

func getCR(ctx context.Context, skrClient client.Client, manifest *v1beta2.Manifest) (*unstructured.Unstructured,
	error,
) {
	resourceCR := &unstructured.Unstructured{}
	name := manifest.Spec.Resource.GetName()
	namespace := manifest.Spec.Resource.GetNamespace()
	gvk := manifest.Spec.Resource.GroupVersionKind()

	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	if err := skrClient.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		return nil, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}

	return resourceCR, nil
}
