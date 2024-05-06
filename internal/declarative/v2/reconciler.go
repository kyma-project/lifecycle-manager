package v2

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
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
	namespaceNotBeRemoved  = "kyma-system"
	CustomResourceManager  = "resource.kyma-project.io/finalizer"
	SyncedOCIRefAnnotation = "sync-oci-ref"
)

func NewFromManager(mgr manager.Manager, prototype Object, requeueIntervals queue.RequeueIntervals,
	metrics *metrics.ManifestMetrics, mandatoryModulesMetrics *metrics.MandatoryModulesMetrics, options ...Option,
) *Reconciler {
	reconciler := &Reconciler{}
	reconciler.prototype = prototype
	reconciler.ManifestMetrics = metrics
	reconciler.MandatoryModuleMetrics = mandatoryModulesMetrics
	reconciler.RequeueIntervals = requeueIntervals
	reconciler.Options = DefaultOptions().Apply(WithManager(mgr)).Apply(options...)
	return reconciler
}

type Reconciler struct {
	prototype Object
	queue.RequeueIntervals
	*Options
	ManifestMetrics        *metrics.ManifestMetrics
	MandatoryModuleMetrics *metrics.MandatoryModulesMetrics
}

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

func newInstallationCondition(obj Object) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeInstallation),
		Reason:             string(ConditionReasonReady),
		Status:             apimetav1.ConditionFalse,
		Message:            "installation is ready and resources can be used",
		ObservedGeneration: obj.GetGeneration(),
	}
}

func newResourcesCondition(obj Object) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeResources),
		Reason:             string(ConditionReasonResourcesAreAvailable),
		Status:             apimetav1.ConditionFalse,
		Message:            "resources are parsed and ready for use",
		ObservedGeneration: obj.GetGeneration(),
	}
}

//nolint:funlen,cyclop,gocognit // Declarative pkg will be removed soon
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	defer r.recordReconciliationDuration(startTime, req.Name)
	obj, ok := r.prototype.DeepCopyObject().(Object)
	if !ok {
		r.ManifestMetrics.RecordRequeueReason(metrics.ManifestTypeCast, queue.UnexpectedRequeue)
		return ctrl.Result{}, common.ErrTypeAssert
	}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if util.IsNotFound(err) {
			logf.FromContext(ctx).Info(req.NamespacedName.String() + " got deleted!")
			return ctrl.Result{}, nil
		}
		r.ManifestMetrics.RecordRequeueReason(metrics.ManifestRetrieval, queue.UnexpectedRequeue)
		return ctrl.Result{}, fmt.Errorf("manifestController: %w", err)
	}
	currentObjStatus := obj.GetStatus()

	if r.ShouldSkip(ctx, obj) {
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	if err := r.initialize(obj); err != nil {
		return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestInit, currentObjStatus, err)
	}

	if obj.GetLabels() != nil && obj.GetLabels()[shared.IsMandatoryModule] == strconv.FormatBool(true) {
		state := obj.GetStatus().State
		kymaName := obj.GetLabels()[shared.KymaName]
		moduleName := obj.GetLabels()[shared.ModuleName]
		r.MandatoryModuleMetrics.RecordMandatoryModuleState(kymaName, moduleName, state)
	}

	if obj.GetDeletionTimestamp().IsZero() {
		objMeta := r.partialObjectMetadata(obj)
		if controllerutil.AddFinalizer(objMeta, r.Finalizer) {
			return r.ssaSpec(ctx, objMeta, metrics.ManifestAddFinalizer)
		}
	}

	spec, err := r.Spec(ctx, obj)
	if err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			r.ManifestMetrics.RemoveManifestDuration(req.Name)
			r.cleanUpMandatoryModuleMetrics(obj)
			return r.removeFinalizers(ctx, obj, []string{r.Finalizer}, metrics.ManifestRemoveFinalizerWhenParseSpec,
				err)
		}
		return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestParseSpec, currentObjStatus, err)
	}

	if notContainsSyncedOCIRefAnnotation(obj) {
		updateSyncedOCIRefAnnotation(obj, spec.OCIRef)
		return r.updateObject(ctx, obj, metrics.ManifestInitSyncedOCIRef, queue.IntendedRequeue)
	}

	clnt, err := r.getTargetClient(ctx, obj)
	if err != nil {
		if !obj.GetDeletionTimestamp().IsZero() && errors.Is(err, ErrAccessSecretNotFound) {
			r.cleanUpMandatoryModuleMetrics(obj)
			r.ManifestMetrics.RemoveManifestDuration(req.Name)
			return r.removeFinalizers(ctx, obj, obj.GetFinalizers(), metrics.ManifestRemoveFinalizerWhenSecretGone, err)
		}

		obj.SetStatus(obj.GetStatus().WithState(shared.StateError).WithErr(err))
		return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestClientInit, currentObjStatus, err)
	}

	target, current, err := r.renderResources(ctx, clnt, obj, spec)
	if err != nil {
		if util.IsConnectionRelatedError(err) {
			r.invalidateClientCache(ctx, obj)
			return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestUnauthorized, currentObjStatus, err)
		}

		return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestRenderResources, currentObjStatus, err)
	}

	if err := r.pruneDiff(ctx, clnt, obj, current, target, spec); errors.Is(err, resources.ErrDeletionNotFinished) {
		r.ManifestMetrics.RecordRequeueReason(metrics.ManifestPruneDiffNotFinished, queue.IntendedRequeue)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestPruneDiff, currentObjStatus, err)
	}

	if err := r.removeModuleCR(ctx, clnt, obj); err != nil {
		if errors.Is(err, ErrRequeueRequired) {
			r.ManifestMetrics.RecordRequeueReason(metrics.ManifestPreDeleteEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestPreDelete, currentObjStatus, err)
	}

	if err = r.syncResources(ctx, clnt, obj, target); err != nil {
		if errors.Is(err, ErrRequeueRequired) {
			r.ManifestMetrics.RecordRequeueReason(metrics.ManifestSyncResourcesEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		if errors.Is(err, ErrClientUnauthorized) {
			r.invalidateClientCache(ctx, obj)
		}
		return r.ssaStatusIfDiffExist(ctx, obj, metrics.ManifestSyncResources, currentObjStatus, err)
	}

	// This situation happens when manifest get new installation layer to update resources,
	// we need to make sure all updates successfully before we can update synced oci ref
	if requireUpdateSyncedOCIRefAnnotation(obj, spec.OCIRef) {
		updateSyncedOCIRefAnnotation(obj, spec.OCIRef)
		return r.updateObject(ctx, obj, metrics.ManifestUpdateSyncedOCIRef, queue.IntendedRequeue)
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		r.cleanUpMandatoryModuleMetrics(obj)
		r.ManifestMetrics.RemoveManifestDuration(req.Name)
		return r.removeFinalizers(ctx, obj, []string{r.Finalizer}, metrics.ManifestRemoveFinalizerInDeleting, nil)
	}

	if err = r.handleStatusPatch(ctx, obj, currentObjStatus); err != nil {
		r.Event(obj, "Warning", "PatchStatus", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to patch status: %w", err)
	}

	return ctrl.Result{RequeueAfter: r.Success}, nil
}

func (r *Reconciler) invalidateClientCache(ctx context.Context, obj Object) {
	if r.ClientCacheKeyFn != nil {
		clientsCacheKey, ok := r.ClientCacheKeyFn(ctx, obj)
		if ok {
			logf.FromContext(ctx).Info("Invalidating manifest-controller client cache entry for key: " + fmt.Sprintf("%#v",
				clientsCacheKey))
			r.ClientCache.DeleteClient(clientsCacheKey)
		}
	}
}

func (r *Reconciler) removeFinalizers(ctx context.Context, obj Object, finalizersToRemove []string,
	requeueReason metrics.ManifestRequeueReason, err error,
) (ctrl.Result, error) {
	currentObjStatus := obj.GetStatus()
	finalizerRemoved := false
	for _, f := range finalizersToRemove {
		if controllerutil.RemoveFinalizer(obj, f) {
			finalizerRemoved = true
		}
	}
	if finalizerRemoved {
		return r.updateObject(ctx, obj, requeueReason, queue.IntendedRequeue)
	}

	if obj.GetStatus().State != shared.StateWarning {
		obj.SetStatus(obj.GetStatus().WithState(shared.StateDeleting).
			WithOperation(fmt.Sprintf("waiting as other finalizers are present: %s", obj.GetFinalizers())))
	}

	return r.ssaStatusIfDiffExist(ctx, obj, requeueReason, currentObjStatus, err)
}

func (r *Reconciler) partialObjectMetadata(obj Object) *apimetav1.PartialObjectMetadata {
	objMeta := &apimetav1.PartialObjectMetadata{}
	objMeta.SetName(obj.GetName())
	objMeta.SetNamespace(obj.GetNamespace())
	objMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	objMeta.SetFinalizers(obj.GetFinalizers())
	return objMeta
}

func (r *Reconciler) initialize(obj Object) error {
	status := obj.GetStatus()

	for _, condition := range []apimetav1.Condition{
		newResourcesCondition(obj),
		newInstallationCondition(obj),
	} {
		if meta.FindStatusCondition(status.Conditions, condition.Type) == nil {
			meta.SetStatusCondition(&status.Conditions, condition)
		}
	}

	if status.Synced == nil {
		status.Synced = []shared.Resource{}
	}

	if status.State == "" {
		obj.SetStatus(status.WithState(shared.StateProcessing).WithErr(ErrObjectHasEmptyState))
		return ErrObjectHasEmptyState
	}

	obj.SetStatus(status)

	return nil
}

func (r *Reconciler) Spec(ctx context.Context, obj Object) (*Spec, error) {
	spec, err := r.SpecResolver.Spec(ctx, obj)
	if err != nil {
		obj.SetStatus(obj.GetStatus().WithState(shared.StateError).WithErr(err))
	}
	return spec, err
}

func (r *Reconciler) renderResources(
	ctx context.Context,
	clnt Client,
	obj Object,
	spec *Spec,
) ([]*resource.Info, []*resource.Info, error) {
	resourceCondition := newResourcesCondition(obj)
	status := obj.GetStatus()

	var err error
	var target, current ResourceList

	converter := NewResourceToInfoConverter(ResourceInfoConverter(clnt), r.Namespace)

	if target, err = r.renderTargetResources(ctx, clnt, converter, obj, spec); err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, nil, err
	}

	current, err = converter.ResourcesToInfos(status.Synced)
	if err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, nil, err
	}

	if !meta.IsStatusConditionTrue(status.Conditions, resourceCondition.Type) {
		resourceCondition.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, resourceCondition)
		obj.SetStatus(status.WithOperation(resourceCondition.Message))
	}

	return target, current, nil
}

func (r *Reconciler) syncResources(ctx context.Context, clnt Client, obj Object,
	target []*resource.Info,
) error {
	status := obj.GetStatus()

	if err := ConcurrentSSA(clnt, r.FieldOwner).Run(ctx, target); err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return err
	}

	oldSynced := status.Synced
	newSynced := NewInfoToResourceConverter().InfosToResources(target)
	status.Synced = newSynced

	if hasDiff(oldSynced, newSynced) {
		if obj.GetDeletionTimestamp().IsZero() {
			obj.SetStatus(status.WithState(shared.StateProcessing).WithOperation(ErrWarningResourceSyncStateDiff.Error()))
		} else if status.State != shared.StateWarning {
			obj.SetStatus(status.WithState(shared.StateDeleting).WithOperation(ErrWarningResourceSyncStateDiff.Error()))
		}
		return ErrWarningResourceSyncStateDiff
	}

	for i := range r.PostRuns {
		if err := r.PostRuns[i](ctx, clnt, r.Client, obj); err != nil {
			obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
			return err
		}
	}

	return r.checkTargetReadiness(ctx, clnt, obj, target)
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

func (r *Reconciler) checkTargetReadiness(
	ctx context.Context, clnt Client, manifest Object, target []*resource.Info,
) error {
	status := manifest.GetStatus()

	resourceReadyCheck := r.CustomReadyCheck

	crStateInfo, err := resourceReadyCheck.Run(ctx, clnt, manifest, target)
	if err != nil {
		manifest.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return err
	}

	if crStateInfo.State == shared.StateProcessing {
		waitingMsg := "waiting for resources to become ready: " + crStateInfo.Info
		manifest.SetStatus(status.WithState(shared.StateProcessing).WithOperation(waitingMsg))
		return ErrInstallationConditionRequiresUpdate
	}

	installationCondition := newInstallationCondition(manifest)
	if !meta.IsStatusConditionTrue(status.Conditions, installationCondition.Type) || status.State != crStateInfo.State {
		installationCondition.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, installationCondition)
		manifest.SetStatus(status.WithState(crStateInfo.State).
			WithOperation(generateOperationMessage(installationCondition, crStateInfo)))
		return ErrInstallationConditionRequiresUpdate
	}

	return nil
}

func generateOperationMessage(installationCondition apimetav1.Condition, stateInfo StateInfo) string {
	if stateInfo.Info != "" {
		return stateInfo.Info
	}
	return installationCondition.Message
}

func (r *Reconciler) removeModuleCR(ctx context.Context, clnt Client, obj Object) error {
	if !obj.GetDeletionTimestamp().IsZero() {
		for _, preDelete := range r.PreDeletes {
			if err := preDelete(ctx, clnt, r.Client, obj); err != nil {
				// we do not set a status here since it will be deleting if timestamp is set.
				obj.SetStatus(obj.GetStatus().WithErr(err))
				return err
			}
		}
	}
	return nil
}

func (r *Reconciler) renderTargetResources(
	ctx context.Context,
	clnt client.Client,
	converter ResourceToInfoConverter,
	obj Object,
	spec *Spec,
) ([]*resource.Info, error) {
	if !obj.GetDeletionTimestamp().IsZero() {
		deleted, err := r.DeletionCheck.Run(ctx, clnt, obj)
		if err != nil {
			return nil, err
		}
		if deleted {
			return ResourceList{}, nil
		}
	}

	status := obj.GetStatus()

	targetResources, err := r.ManifestParser.Parse(spec)
	if err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, err
	}

	for _, transform := range r.PostRenderTransforms {
		if err := transform(ctx, obj, targetResources.Items); err != nil {
			obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
			return nil, err
		}
	}

	target, err := converter.UnstructuredToInfos(targetResources.Items)
	if err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, err
	}

	return target, nil
}

func (r *Reconciler) pruneDiff(
	ctx context.Context,
	clnt Client,
	obj Object,
	current, target []*resource.Info,
	spec *Spec,
) error {
	diff, err := pruneResource(ResourceList(current).Difference(target), "Namespace", namespaceNotBeRemoved)
	if err != nil {
		obj.SetStatus(obj.GetStatus().WithErr(err))
		return err
	}
	if len(diff) == 0 {
		return nil
	}
	if manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff, obj, spec) {
		// This case should not happen normally, but if happens, it means the resources read from cache is incomplete,
		// and we should prevent diff resources to be deleted.
		// Meanwhile, evict cache to hope newly created resources back to normal.
		obj.SetStatus(obj.GetStatus().WithState(shared.StateWarning).WithOperation(ErrResourceSyncDiffInSameOCILayer.Error()))
		r.ManifestParser.EvictCache(spec)
		return ErrResourceSyncDiffInSameOCILayer
	}

	// Remove this type casting while in progress this issue: https://github.com/kyma-project/lifecycle-manager/issues/1006
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		obj.SetStatus(obj.GetStatus().WithErr(v1beta2.ErrTypeAssertManifest))
		return v1beta2.ErrTypeAssertManifest
	}
	err = resources.NewConcurrentCleanup(clnt, manifest).DeleteDiffResources(ctx, diff)
	if err != nil {
		obj.SetStatus(obj.GetStatus().WithErr(err))
	}

	return err
}

func manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff []*resource.Info, obj Object,
	spec *Spec,
) bool {
	return len(diff) > 0 && ociRefNotChanged(obj, spec.OCIRef) && obj.GetDeletionTimestamp().IsZero()
}

func ociRefNotChanged(obj Object, ref string) bool {
	syncedOCIRef, found := obj.GetAnnotations()[SyncedOCIRefAnnotation]
	return found && syncedOCIRef == ref
}

func requireUpdateSyncedOCIRefAnnotation(obj Object, ref string) bool {
	syncedOCIRef, found := obj.GetAnnotations()[SyncedOCIRefAnnotation]
	if found && syncedOCIRef != ref {
		return true
	}
	return false
}

func notContainsSyncedOCIRefAnnotation(obj Object) bool {
	_, found := obj.GetAnnotations()[SyncedOCIRefAnnotation]
	return !found
}

func updateSyncedOCIRefAnnotation(obj Object, ref string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[SyncedOCIRefAnnotation] = ref
	obj.SetAnnotations(annotations)
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

func (r *Reconciler) getTargetClient(ctx context.Context, obj Object) (Client, error) {
	var err error
	var clnt Client
	if r.ClientCacheKeyFn == nil {
		return r.configClient(ctx, obj)
	}

	clientsCacheKey, found := r.ClientCacheKeyFn(ctx, obj)
	if found {
		clnt = r.GetClient(clientsCacheKey)
	}

	if clnt == nil {
		clnt, err = r.configClient(ctx, obj)
		if err != nil {
			return nil, err
		}
		r.AddClient(clientsCacheKey, clnt)
	}

	if r.Namespace != apimetav1.NamespaceNone && r.Namespace != apimetav1.NamespaceDefault {
		err := clnt.Patch(
			ctx, &apicorev1.Namespace{
				TypeMeta:   apimetav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: apimetav1.ObjectMeta{Name: r.Namespace},
			}, client.Apply, client.ForceOwnership, r.FieldOwner,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to patch namespace: %w", err)
		}
	}

	return clnt, nil
}

func (r *Reconciler) configClient(ctx context.Context, obj Object) (Client, error) {
	var err error

	cluster := &ClusterInfo{
		Config: r.Config,
		Client: r.Client,
	}

	if r.TargetCluster != nil {
		cluster, err = r.TargetCluster(ctx, obj)
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

func (r *Reconciler) ssaStatusIfDiffExist(ctx context.Context, obj Object,
	requeueReason metrics.ManifestRequeueReason, previousStatus shared.Status, originalErr error,
) (ctrl.Result, error) {
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.UnexpectedRequeue)

	if err := r.handleStatusPatch(ctx, obj, previousStatus); err != nil {
		r.Event(obj, "Warning", "PatchStatus", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to patch status: %w", err)
	}

	return ctrl.Result{RequeueAfter: r.RequeueIntervals.Busy}, originalErr
}

func (r *Reconciler) handleStatusPatch(ctx context.Context, obj Object, previousStatus shared.Status) error {
	if hasStatusDiff(obj.GetStatus(), previousStatus) {
		resetNonPatchableField(obj)
		if err := r.Status().Patch(ctx, obj, client.Apply, client.ForceOwnership, r.FieldOwner); err != nil {
			return fmt.Errorf("failed to patch status: %w", err)
		}
	}

	return nil
}

func hasStatusDiff(first, second shared.Status) bool {
	return first.State != second.State || first.LastOperation.Operation != second.LastOperation.Operation
}

func (r *Reconciler) ssaSpec(ctx context.Context, obj client.Object,
	requeueReason metrics.ManifestRequeueReason,
) (ctrl.Result, error) {
	resetNonPatchableField(obj)
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
	if err := r.Patch(ctx, obj, client.Apply, client.ForceOwnership, r.FieldOwner); err != nil {
		r.Event(obj, "Warning", "PatchObject", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to patch object: %w", err)
	}
	return ctrl.Result{Requeue: true}, nil
}

func resetNonPatchableField(obj client.Object) {
	obj.SetUID("")
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
}

func (r *Reconciler) updateObject(ctx context.Context, obj client.Object,
	requeueReason metrics.ManifestRequeueReason,
	requeueType queue.RequeueType,
) (ctrl.Result, error) {
	r.ManifestMetrics.RecordRequeueReason(requeueReason, requeueType)
	if err := r.Update(ctx, obj); err != nil {
		r.Event(obj, "Warning", "UpdateObject", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to update object: %w", err)
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

func (r *Reconciler) cleanUpMandatoryModuleMetrics(obj Object) {
	if obj.GetLabels()[shared.IsMandatoryModule] == strconv.FormatBool(true) {
		kymaName := obj.GetLabels()[shared.KymaName]
		moduleName := obj.GetLabels()[shared.ModuleName]
		r.MandatoryModuleMetrics.CleanupMetrics(kymaName, moduleName)
	}
}
