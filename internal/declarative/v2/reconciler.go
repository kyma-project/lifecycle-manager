package v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	ErrWarningResourceSyncStateDiff              = errors.New("resource syncTarget state diff detected")
	ErrResourceSyncDiffInSameOCILayer            = errors.New("resource syncTarget diff detected but in same oci layer, prevent sync resource to be deleted") //nolint:lll
	ErrInstallationConditionRequiresUpdate       = errors.New("installation condition needs an update")
	ErrDeletionTimestampSetButNotInDeletingState = errors.New("resource is not set to deleting yet")
	ErrObjectHasEmptyState                       = errors.New("object has an empty state")
	ErrKubeconfigFetchFailed                     = errors.New("could not fetch kubeconfig")
)

const (
	namespaceNotBeRemoved  = "kyma-system"
	CustomResourceManager  = "resource.kyma-project.io/finalizer"
	SyncedOCIRefAnnotation = "sync-oci-ref"
)

func NewFromManager(mgr manager.Manager, prototype Object, options ...Option) *Reconciler {
	r := &Reconciler{}
	r.prototype = prototype
	r.Options = DefaultOptions().Apply(WithManager(mgr)).Apply(options...)
	return r
}

type Reconciler struct {
	prototype Object
	*Options
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

func newInstallationCondition(obj Object) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionTypeInstallation),
		Reason:             string(ConditionReasonReady),
		Status:             metav1.ConditionFalse,
		Message:            "installation is ready and resources can be used",
		ObservedGeneration: obj.GetGeneration(),
	}
}

func newResourcesCondition(obj Object) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionTypeResources),
		Reason:             string(ConditionReasonResourcesAreAvailable),
		Status:             metav1.ConditionFalse,
		Message:            "resources are parsed and ready for use",
		ObservedGeneration: obj.GetGeneration(),
	}
}

//nolint:funlen,cyclop
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.FromContext(ctx).Info(fmt.Sprintf("Starting manifest reconciliation at: %s", time.Now()))
	obj, ok := r.prototype.DeepCopyObject().(Object)
	if !ok {
		return ctrl.Result{}, common.ErrTypeAssert
	}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		log.FromContext(ctx).Info(req.NamespacedName.String() + " got deleted!")
		if !util.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("manifestController: %w", err)
		}
		return ctrl.Result{}, nil
	}

	if r.ShouldSkip(ctx, obj) {
		return ctrl.Result{}, nil
	}

	if err := r.initialize(obj); err != nil {
		return r.ssaStatus(ctx, obj)
	}

	if obj.GetDeletionTimestamp().IsZero() {
		objMeta := r.partialObjectMetadata(obj)
		if controllerutil.AddFinalizer(objMeta, r.Finalizer) {
			return r.ssa(ctx, objMeta)
		}
	}

	spec, err := r.Spec(ctx, obj)
	if err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			return r.removeFinalizers(ctx, obj, []string{r.Finalizer})
		}
		return r.ssaStatus(ctx, obj)
	}

	if notContainsSyncedOCIRefAnnotation(obj) {
		updateSyncedOCIRefAnnotation(obj, spec.OCIRef)
		return ctrl.Result{Requeue: true}, r.Update(ctx, obj) //nolint:wrapcheck
	}

	clnt, err := r.getTargetClient(ctx, obj)
	if err != nil {
		if !obj.GetDeletionTimestamp().IsZero() && errors.Is(err, ErrKubeconfigFetchFailed) {
			return r.removeFinalizers(ctx, obj, []string{r.Finalizer, CustomResourceManager})
		}
		r.Event(obj, "Warning", "ClientInitialization", err.Error())
		obj.SetStatus(obj.GetStatus().WithState(StateError).WithErr(err))
		return r.ssaStatus(ctx, obj)
	}

	converter := NewResourceToInfoConverter(clnt, r.Namespace)

	renderer, err := InitializeRenderer(ctx, obj, spec, r.Options)
	if err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			return r.removeFinalizers(ctx, obj, []string{r.Finalizer})
		}
		return r.ssaStatus(ctx, obj)
	}

	target, current, err := r.renderResources(ctx, obj, spec, converter)
	if err != nil {
		return r.ssaStatus(ctx, obj)
	}

	diff := ResourceList(current).Difference(target)
	if err := r.pruneDiff(ctx, clnt, obj, renderer, diff, spec); errors.Is(err, ErrDeletionNotFinished) {
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		return r.ssaStatus(ctx, obj)
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		return r.removeFinalizers(ctx, obj, []string{r.Finalizer})
	}
	if err := r.syncResources(ctx, clnt, obj, target); err != nil {
		return r.ssaStatus(ctx, obj)
	}

	// This situation happens when manifest get new installation layer to update resources,
	// we need to make sure all updates successfully before we can update synced oci ref
	if requireUpdateSyncedOCIRefAnnotation(obj, spec.OCIRef) {
		updateSyncedOCIRefAnnotation(obj, spec.OCIRef)
		return ctrl.Result{Requeue: true}, r.Update(ctx, obj) //nolint:wrapcheck
	}
	return r.CtrlOnSuccess, nil
}

func (r *Reconciler) removeFinalizers(ctx context.Context, obj Object, finalizersToRemove []string) (
	ctrl.Result, error,
) {
	finalizerRemoved := false
	for _, f := range finalizersToRemove {
		if controllerutil.RemoveFinalizer(obj, f) {
			finalizerRemoved = true
		}
	}
	if finalizerRemoved {
		// no SSA since delete does not work for finalizers
		return ctrl.Result{}, r.Update(ctx, obj) //nolint:wrapcheck
	}
	msg := fmt.Sprintf("waiting as other finalizers are present: %s", obj.GetFinalizers())
	r.Event(obj, "Normal", "FinalizerRemoval", msg)
	obj.SetStatus(obj.GetStatus().WithState(StateDeleting).WithOperation(msg))
	return r.ssaStatus(ctx, obj)
}

func (r *Reconciler) partialObjectMetadata(obj Object) *metav1.PartialObjectMetadata {
	objMeta := &metav1.PartialObjectMetadata{}
	objMeta.SetName(obj.GetName())
	objMeta.SetNamespace(obj.GetNamespace())
	objMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	objMeta.SetFinalizers(obj.GetFinalizers())
	return objMeta
}

func (r *Reconciler) initialize(obj Object) error {
	status := obj.GetStatus()

	if !obj.GetDeletionTimestamp().IsZero() && obj.GetStatus().State != StateDeleting {
		obj.SetStatus(status.WithState(StateDeleting).WithErr(ErrDeletionTimestampSetButNotInDeletingState))
		return ErrDeletionTimestampSetButNotInDeletingState
	}

	for _, condition := range []metav1.Condition{
		newResourcesCondition(obj),
		newInstallationCondition(obj),
	} {
		if meta.FindStatusCondition(status.Conditions, condition.Type) == nil {
			meta.SetStatusCondition(&status.Conditions, condition)
		}
	}

	if status.Synced == nil {
		status.Synced = []Resource{}
	}

	if status.State == "" {
		obj.SetStatus(status.WithState(StateProcessing).WithErr(ErrObjectHasEmptyState))
		return ErrObjectHasEmptyState
	}

	obj.SetStatus(status)

	return nil
}

func (r *Reconciler) Spec(ctx context.Context, obj Object) (*Spec, error) {
	targetClient, err := r.getTargetClient(ctx, obj)
	if err != nil {
		return nil, err
	}
	spec, err := r.SpecResolver.Spec(ctx, obj, targetClient)
	if err != nil {
		r.Event(obj, "Warning", "Spec", err.Error())
		obj.SetStatus(obj.GetStatus().WithState(StateError).WithErr(err))
	}
	return spec, err
}

func (r *Reconciler) renderResources(
	ctx context.Context,
	obj Object,
	spec *Spec,
	converter ResourceToInfoConverter,
) ([]*resource.Info, []*resource.Info, error) {
	resourceCondition := newResourcesCondition(obj)
	status := obj.GetStatus()

	var err error
	var target, current ResourceList

	if target, err = r.renderTargetResources(ctx, converter, obj, spec); err != nil {
		return nil, nil, err
	}

	current, err = converter.ResourcesToInfos(status.Synced)
	if err != nil {
		r.Event(obj, "Warning", "CurrentResourceParsing", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, nil, err
	}

	if !meta.IsStatusConditionTrue(status.Conditions, resourceCondition.Type) {
		r.Event(obj, "Normal", resourceCondition.Reason, resourceCondition.Message)
		resourceCondition.Status = metav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, resourceCondition)
		obj.SetStatus(status.WithOperation(resourceCondition.Message))
	}

	return target, current, nil
}

func (r *Reconciler) syncResources(ctx context.Context, clnt Client, obj Object, target []*resource.Info) error {
	status := obj.GetStatus()

	if err := ConcurrentSSA(clnt, r.FieldOwner).Run(ctx, target); err != nil {
		r.Event(obj, "Warning", "ServerSideApply", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}

	oldSynced := status.Synced
	newSynced := NewInfoToResourceConverter().InfosToResources(target)
	status.Synced = newSynced

	if hasDiff(oldSynced, newSynced) {
		obj.SetStatus(status.WithState(StateProcessing).WithOperation(ErrWarningResourceSyncStateDiff.Error()))
		return ErrWarningResourceSyncStateDiff
	}

	for i := range r.PostRuns {
		if err := r.PostRuns[i](ctx, clnt, r.Client, obj); err != nil {
			r.Event(obj, "Warning", "PostRun", err.Error())
			obj.SetStatus(status.WithState(StateError).WithErr(err))
			return err
		}
	}

	return r.checkTargetReadiness(ctx, clnt, obj, target)
}

func hasDiff(oldResources []Resource, newResources []Resource) bool {
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
		r.Event(manifest, "Warning", "ResourceReadyCheck", err.Error())
		manifest.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}

	if crStateInfo.State == StateProcessing {
		waitingMsg := fmt.Sprintf("waiting for resources to become ready: %s", crStateInfo.Info)
		r.Event(manifest, "Normal", "ResourceReadyCheck", waitingMsg)
		manifest.SetStatus(status.WithState(StateProcessing).WithOperation(waitingMsg))
		return ErrInstallationConditionRequiresUpdate
	}

	if crStateInfo.State != StateReady && crStateInfo.State != StateWarning {
		// should not happen, if happens, skip status update
		return nil
	}

	installationCondition := newInstallationCondition(manifest)
	if !meta.IsStatusConditionTrue(status.Conditions, installationCondition.Type) || status.State != crStateInfo.State {
		r.Event(manifest, "Normal", installationCondition.Reason, installationCondition.Message)
		installationCondition.Status = metav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, installationCondition)
		manifest.SetStatus(status.WithState(crStateInfo.State).
			WithOperation(generateOperationMessage(installationCondition, crStateInfo)))
		return ErrInstallationConditionRequiresUpdate
	}

	return nil
}

func generateOperationMessage(installationCondition metav1.Condition, stateInfo StateInfo) string {
	if stateInfo.Info != "" {
		return stateInfo.Info
	}
	return installationCondition.Message
}

func (r *Reconciler) deleteDiffResources(
	ctx context.Context, clnt Client, obj Object, diff []*resource.Info,
) error {
	status := obj.GetStatus()

	if err := NewConcurrentCleanup(clnt).Run(ctx, diff); errors.Is(err, ErrDeletionNotFinished) {
		r.Event(obj, "Normal", "Deletion", err.Error())
		return err
	} else if err != nil {
		r.Event(obj, "Warning", "Deletion", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return err
	}

	return nil
}

func (r *Reconciler) doPreDelete(ctx context.Context, clnt Client, obj Object) error {
	if !obj.GetDeletionTimestamp().IsZero() {
		for _, preDelete := range r.PreDeletes {
			if err := preDelete(ctx, clnt, r.Client, obj); err != nil {
				r.Event(obj, "Warning", "PreDelete", err.Error())
				// we do not set a status here since it will be deleting if timestamp is set.
				return err
			}
		}
	}
	return nil
}

func (r *Reconciler) renderTargetResources(
	ctx context.Context,
	converter ResourceToInfoConverter,
	obj Object,
	spec *Spec,
) ([]*resource.Info, error) {
	if !obj.GetDeletionTimestamp().IsZero() {
		// if we are deleting the resources,
		// we no longer want to have any target resources and want to clean up all existing resources.
		// Thus, we empty the target here so the difference will be the entire current
		// resource list in the cluster.
		return ResourceList{}, nil
	}

	status := obj.GetStatus()

	targetResources, err := r.ManifestParser.Parse(spec)
	if err != nil {
		r.Event(obj, "Warning", "ManifestParsing", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, err
	}

	for _, transform := range r.PostRenderTransforms {
		if err := transform(ctx, obj, targetResources.Items); err != nil {
			r.Event(obj, "Warning", "PostRenderTransform", err.Error())
			obj.SetStatus(status.WithState(StateError).WithErr(err))
			return nil, err
		}
	}

	target, err := converter.UnstructuredToInfos(targetResources.Items)
	if err != nil {
		r.Event(obj, "Warning", "TargetResourceParsing", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, err
	}

	return target, nil
}

func (r *Reconciler) pruneDiff(
	ctx context.Context,
	clnt Client,
	obj Object,
	renderer Renderer,
	diff []*resource.Info,
	spec *Spec,
) error {
	var err error
	diff, err = pruneResource(diff, "Namespace", namespaceNotBeRemoved)
	if err != nil {
		return err
	}
	resourceName := r.ModuleCRDName(obj)
	diff, err = pruneResource(diff, "CustomResourceDefinition", resourceName)
	if err != nil {
		return err
	}

	if manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff, obj, spec) {
		// This case should not happen normally, but if happens, it means the resources read from cache is incomplete,
		// and we should prevent diff resources to be deleted.
		// Meanwhile, evict cache to hope newly created resources back to normal.
		r.Event(obj, "Warning", "PruneDiff", ErrResourceSyncDiffInSameOCILayer.Error())
		obj.SetStatus(obj.GetStatus().WithState(StateWarning).WithOperation(ErrResourceSyncDiffInSameOCILayer.Error()))
		r.ManifestParser.EvictCache(spec)
		return ErrResourceSyncDiffInSameOCILayer
	}

	if err := r.doPreDelete(ctx, clnt, obj); err != nil {
		return err
	}

	if err := r.deleteDiffResources(ctx, clnt, obj, diff); err != nil {
		return err
	}

	if obj.GetDeletionTimestamp().IsZero() || !r.DeletePrerequisites {
		return nil
	}

	return renderer.RemovePrerequisites(ctx, obj)
}

func manifestNotInDeletingAndOciRefNotChangedButDiffDetected(diff []*resource.Info, obj Object, spec *Spec) bool {
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
	//nolint:varnamelen
	for i, info := range diff {
		obj, ok := info.Object.(client.Object)
		if !ok {
			return diff, common.ErrTypeAssert
		}
		if obj.GetObjectKind().GroupVersionKind().Kind == resourceType && obj.GetName() == resourceName {
			return append(diff[:i], diff[i+1:]...), nil
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
		clnt = r.GetClientFromCache(clientsCacheKey)
	}

	if clnt == nil {
		clnt, err = r.configClient(ctx, obj)
		if err != nil {
			return nil, err
		}
		r.SetClientInCache(clientsCacheKey, clnt)
	}

	if r.Namespace != metav1.NamespaceNone && r.Namespace != metav1.NamespaceDefault {
		err := clnt.Patch(
			ctx, &v1.Namespace{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{Name: r.Namespace},
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

func (r *Reconciler) ssaStatus(ctx context.Context, obj client.Object) (ctrl.Result, error) {
	obj.SetUID("")
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	// TODO: replace the SubResourcePatchOptions with  client.ForceOwnership, r.FieldOwner in later compatible version
	return ctrl.Result{Requeue: true}, r.Status().Patch( //nolint:wrapcheck
		ctx, obj, client.Apply, subResourceOpts(client.ForceOwnership, r.FieldOwner),
	)
}

func subResourceOpts(opts ...client.PatchOption) client.SubResourcePatchOption {
	return &client.SubResourcePatchOptions{PatchOptions: *(&client.PatchOptions{}).ApplyOptions(opts)}
}

func (r *Reconciler) ssa(ctx context.Context, obj client.Object) (ctrl.Result, error) {
	obj.SetUID("")
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return ctrl.Result{Requeue: true},
		r.Patch(ctx, obj, //nolint:wrapcheck
			client.Apply,
			client.ForceOwnership,
			r.FieldOwner)
}
