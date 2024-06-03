package v2

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strconv"
	"time"

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
	namespaceNotBeRemoved                    = "kyma-system"
	CustomResourceManager                    = "resource.kyma-project.io/finalizer"
	SyncedOCIRefAnnotation                   = "sync-oci-ref"
	finalizerDefault                         = "declarative.kyma-project.io/finalizer"
	fieldOwnerDefault      client.FieldOwner = "declarative.kyma-project.io/applier"
)

const (
	OperatorName              = "module-manager"
	ManagedByLabelValue       = "declarative-v2"
	DisclaimerAnnotation      = shared.OperatorGroup + shared.Separator + "managed-by-reconciler-disclaimer"
	DisclaimerAnnotationValue = "DO NOT EDIT - This resource is managed by Kyma.\n" +
		"Any modifications are discarded and the resource is reverted to the original state."
)

func NewFromManager(mgr manager.Manager,
	prototype Object,
	requeueIntervals queue.RequeueIntervals,
	metrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestParser ManifestParser,
	crReadyCheck ReadyCheck,
	options ...Option,
) *Reconciler {
	return &Reconciler{
		prototype:              prototype,
		RequeueIntervals:       requeueIntervals,
		ManifestMetrics:        metrics,
		MandatoryModuleMetrics: mandatoryModulesMetrics,
		manifestParser:         manifestParser,
		crReadyCheck:           crReadyCheck,
		Options:                DefaultOptions().Apply(WithManager(mgr)).Apply(options...),
	}
}

type Reconciler struct {
	prototype Object
	queue.RequeueIntervals
	*Options
	ManifestMetrics        *metrics.ManifestMetrics
	MandatoryModuleMetrics *metrics.MandatoryModulesMetrics
	manifestParser         ManifestParser
	crReadyCheck           ReadyCheck
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

func (r *Reconciler) shouldSkipReconcile(ctx context.Context, object Object) bool {
	if object.GetLabels() != nil && object.GetLabels()[shared.SkipReconcileLabel] == strconv.FormatBool(true) {
		logf.FromContext(ctx, "skip-label", shared.SkipReconcileLabel).
			V(internal.DebugLogLevel).Info("resource gets skipped because of label")
		return true
	}

	return false
}

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

	if r.shouldSkipReconcile(ctx, obj) {
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	if err := r.initialize(obj); err != nil {
		return r.finishReconcile(ctx, obj, metrics.ManifestInit, currentObjStatus, err)
	}

	if obj.GetLabels() != nil && obj.GetLabels()[shared.IsMandatoryModule] == strconv.FormatBool(true) {
		state := obj.GetStatus().State
		kymaName := obj.GetLabels()[shared.KymaName]
		moduleName := obj.GetLabels()[shared.ModuleName]
		r.MandatoryModuleMetrics.RecordMandatoryModuleState(kymaName, moduleName, state)
	}

	if obj.GetDeletionTimestamp().IsZero() {
		objMeta := r.partialObjectMetadata(obj)
		if controllerutil.AddFinalizer(objMeta, finalizerDefault) {
			return r.ssaSpec(ctx, objMeta, metrics.ManifestAddFinalizer)
		}
	}

	spec, err := r.Spec(ctx, obj)
	if err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			return r.cleanupManifest(ctx, req, obj, currentObjStatus, metrics.ManifestParseSpec, err)
		}
		return r.finishReconcile(ctx, obj, metrics.ManifestParseSpec, currentObjStatus, err)
	}

	if notContainsSyncedOCIRefAnnotation(obj) {
		updateSyncedOCIRefAnnotation(obj, spec.OCIRef)
		return r.updateObject(ctx, obj, metrics.ManifestInitSyncedOCIRef)
	}

	clnt, err := r.getTargetClient(ctx, obj)
	if err != nil {
		if !obj.GetDeletionTimestamp().IsZero() && errors.Is(err, ErrAccessSecretNotFound) {
			return r.cleanupManifest(ctx, req, obj, currentObjStatus, metrics.ManifestClientInit,
				err)
		}

		obj.SetStatus(obj.GetStatus().WithState(shared.StateError).WithErr(err))
		return r.finishReconcile(ctx, obj, metrics.ManifestClientInit, currentObjStatus, err)
	}

	target, current, err := r.renderResources(clnt, obj, spec)
	if err != nil {
		if util.IsConnectionRelatedError(err) {
			r.invalidateClientCache(ctx, obj)
			return r.finishReconcile(ctx, obj, metrics.ManifestUnauthorized, currentObjStatus, err)
		}

		return r.finishReconcile(ctx, obj, metrics.ManifestRenderResources, currentObjStatus, err)
	}

	if err := r.pruneDiff(ctx, clnt, obj, current, target, spec); errors.Is(err, resources.ErrDeletionNotFinished) {
		r.ManifestMetrics.RecordRequeueReason(metrics.ManifestPruneDiffNotFinished, queue.IntendedRequeue)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		return r.finishReconcile(ctx, obj, metrics.ManifestPruneDiff, currentObjStatus, err)
	}

	if err := r.removeModuleCR(ctx, clnt, obj); err != nil {
		if errors.Is(err, ErrRequeueRequired) {
			r.ManifestMetrics.RecordRequeueReason(metrics.ManifestPreDeleteEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		return r.finishReconcile(ctx, obj, metrics.ManifestPreDelete, currentObjStatus, err)
	}

	if err = r.syncResources(ctx, clnt, obj, target); err != nil {
		if errors.Is(err, ErrRequeueRequired) {
			r.ManifestMetrics.RecordRequeueReason(metrics.ManifestSyncResourcesEnqueueRequired, queue.IntendedRequeue)
			return ctrl.Result{Requeue: true}, nil
		}
		if errors.Is(err, ErrClientUnauthorized) {
			r.invalidateClientCache(ctx, obj)
		}
		return r.finishReconcile(ctx, obj, metrics.ManifestSyncResources, currentObjStatus, err)
	}

	// This situation happens when manifest get new installation layer to update resources,
	// we need to make sure all updates successfully before we can update synced oci ref
	if requireUpdateSyncedOCIRefAnnotation(obj, spec.OCIRef) {
		updateSyncedOCIRefAnnotation(obj, spec.OCIRef)
		return r.updateObject(ctx, obj, metrics.ManifestUpdateSyncedOCIRef)
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		return r.cleanupManifest(ctx, req, obj, currentObjStatus, metrics.ManifestReconcileFinished, nil)
	}

	return r.finishReconcile(ctx, obj, metrics.ManifestReconcileFinished, currentObjStatus, nil)
}

func (r *Reconciler) cleanupManifest(ctx context.Context, req ctrl.Request, obj Object, currentObjStatus shared.Status,
	requeueReason metrics.ManifestRequeueReason, originalErr error,
) (ctrl.Result, error) {
	r.ManifestMetrics.RemoveManifestDuration(req.Name)
	r.cleanUpMandatoryModuleMetrics(obj)
	if removeFinalizers(obj, r.finalizerToRemove(originalErr, obj)) {
		return r.updateObject(ctx, obj, requeueReason)
	}
	if obj.GetStatus().State != shared.StateWarning {
		obj.SetStatus(obj.GetStatus().WithState(shared.StateDeleting).
			WithOperation(fmt.Sprintf("waiting as other finalizers are present: %s", obj.GetFinalizers())))
	}
	return r.finishReconcile(ctx, obj, requeueReason, currentObjStatus, originalErr)
}

func (r *Reconciler) finalizerToRemove(originalErr error, obj Object) []string {
	finalizersToRemove := []string{finalizerDefault}
	if errors.Is(originalErr, ErrAccessSecretNotFound) {
		finalizersToRemove = obj.GetFinalizers()
	}
	return finalizersToRemove
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

func removeFinalizers(obj Object, finalizersToRemove []string) bool {
	finalizerRemoved := false
	for _, f := range finalizersToRemove {
		if controllerutil.RemoveFinalizer(obj, f) {
			finalizerRemoved = true
		}
	}

	return finalizerRemoved
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
	clnt Client,
	obj Object,
	spec *Spec,
) ([]*resource.Info, []*resource.Info, error) {
	resourceCondition := newResourcesCondition(obj)
	status := obj.GetStatus()

	var err error
	var target, current ResourceList

	converter := NewResourceToInfoConverter(ResourceInfoConverter(clnt), apimetav1.NamespaceDefault)

	if target, err = r.renderTargetResources(converter, obj, spec); err != nil {
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

	if err := ConcurrentSSA(clnt, fieldOwnerDefault).Run(ctx, target); err != nil {
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

	if err := r.doPostRun(ctx, clnt, obj); err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return err
	}

	return r.checkTargetReadiness(ctx, clnt, obj, target)
}

func (r *Reconciler) doPostRun(ctx context.Context, skrClient Client, obj Object) error {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil
	}
	if manifest.Spec.Resource == nil {
		return nil
	}
	if !manifest.GetDeletionTimestamp().IsZero() {
		return nil
	}

	res := manifest.Spec.Resource.DeepCopy()
	err := skrClient.Create(ctx, res, client.FieldOwner(CustomResourceManager))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	oMeta := &apimetav1.PartialObjectMetadata{}
	oMeta.SetName(obj.GetName())
	oMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	oMeta.SetNamespace(obj.GetNamespace())
	oMeta.SetFinalizers(obj.GetFinalizers())
	if added := controllerutil.AddFinalizer(oMeta, CustomResourceManager); added {
		if err := r.Client.Patch(
			ctx, oMeta, client.Apply, client.ForceOwnership, client.FieldOwner(CustomResourceManager),
		); err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
		return ErrRequeueRequired
	}
	return nil
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

	crStateInfo, err := r.crReadyCheck.Run(ctx, clnt, manifest, target)
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
		if err := r.preDeleteDeleteCR(ctx, clnt, obj); err != nil {
			// we do not set a status here since it will be deleting if timestamp is set.
			obj.SetStatus(obj.GetStatus().WithErr(err))
			return err
		}

	}
	return nil
}

// preDeleteDeleteCR is a hook for deleting the module CR if available in the cluster.
// It uses DeletePropagationBackground to delete module CR.
// Only if module CR is not found (indicated by NotFound error), it continues to remove Manifest finalizer,
// and we consider the CR removal successful.
func (r *Reconciler) preDeleteDeleteCR(
	ctx context.Context, skr Client, obj Object,
) error {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil
	}
	if manifest.Spec.Resource == nil {
		return nil
	}

	resourceCopy := manifest.Spec.Resource.DeepCopy()
	propagation := apimetav1.DeletePropagationBackground
	err := skr.Delete(ctx, resourceCopy, &client.DeleteOptions{PropagationPolicy: &propagation})

	if !util.IsNotFound(err) {
		return nil
	}

	onCluster := manifest.DeepCopy()
	err = r.Client.Get(ctx, client.ObjectKeyFromObject(obj), onCluster)
	if util.IsNotFound(err) {
		return fmt.Errorf("PreDeleteDeleteCR: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}
	if removed := controllerutil.RemoveFinalizer(onCluster, CustomResourceManager); removed {
		if err := r.Client.Update(
			ctx, onCluster, client.FieldOwner(CustomResourceManager),
		); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
		return ErrRequeueRequired
	}
	return nil
}

func (r *Reconciler) renderTargetResources(
	converter ResourceToInfoConverter,
	obj Object,
	spec *Spec,
) ([]*resource.Info, error) {
	if !obj.GetDeletionTimestamp().IsZero() {
		return ResourceList{}, nil
	}

	status := obj.GetStatus()

	targetResources, err := r.manifestParser.Parse(spec)
	if err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, err
	}

	doPostRenderTransforms(obj, targetResources.Items)

	target, err := converter.UnstructuredToInfos(targetResources.Items)
	if err != nil {
		obj.SetStatus(status.WithState(shared.StateError).WithErr(err))
		return nil, err
	}

	return target, nil
}

func doPostRenderTransforms(obj Object, resources []*unstructured.Unstructured) {
	for _, res := range resources {
		lbls := res.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}

		lbls[shared.ManagedBy] = ManagedByLabelValue
		lbls[shared.WatchedByLabel] = OperatorName
		lbls["app.kubernetes.io/component"] = obj.GetName()
		lbls["app.kubernetes.io/part-of"] = "Kyma"
		res.SetLabels(lbls)

		// TODO this part actually has no effect because we didn't set annotations in default transforms
		//annotations := resource.GetAnnotations()
		//if annotations == nil {
		//	annotations = make(map[string]string)
		//}
		//annotations[shared.OwnedByAnnotation] = fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())

		annotations := res.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[DisclaimerAnnotation] = DisclaimerAnnotationValue
		res.SetAnnotations(annotations)
	}
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
		r.manifestParser.EvictCache(spec)
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

func (r *Reconciler) finishReconcile(ctx context.Context, obj Object,
	requeueReason metrics.ManifestRequeueReason, previousStatus shared.Status, originalErr error,
) (ctrl.Result, error) {
	if err := r.patchStatusIfDiffExist(ctx, obj, previousStatus); err != nil {
		r.Event(obj, "Warning", "PatchStatus", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to patch status: %w", err)
	}
	if originalErr != nil {
		r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.UnexpectedRequeue)
		return ctrl.Result{}, originalErr
	}
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
	return ctrl.Result{RequeueAfter: r.Success}, nil
}

func (r *Reconciler) patchStatusIfDiffExist(ctx context.Context, obj Object, previousStatus shared.Status) error {
	if hasStatusDiff(obj.GetStatus(), previousStatus) {
		resetNonPatchableField(obj)
		if err := r.Status().Patch(ctx, obj, client.Apply, client.ForceOwnership, fieldOwnerDefault); err != nil {
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
	if err := r.Patch(ctx, obj, client.Apply, client.ForceOwnership, fieldOwnerDefault); err != nil {
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
) (ctrl.Result, error) {
	r.ManifestMetrics.RecordRequeueReason(requeueReason, queue.IntendedRequeue)
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
