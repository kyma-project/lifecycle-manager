package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrNotFoundAndKCPKymaUnderDeleting = errors.New("not found and kcp kyma under deleting")

const (
	fieldManager = "kyma-sync-context"

	crdInstallation     event.Reason = "CRDInstallation"
	remoteInstallation  event.Reason = "RemoteInstallation"
	metadataSyncFailure event.Reason = "MetadataSynchronization"
	statusSyncFailure   event.Reason = "StatusSynchronization"
)

type SkrContext struct {
	Client

	event event.Event
}

func NewSkrContext(client Client, event event.Event) *SkrContext {
	return &SkrContext{
		Client: client,
		event:  event,
	}
}

func (s *SkrContext) RemoveFinalizersFromKyma(ctx context.Context) error {
	remoteKyma, err := s.getRemoteKyma(ctx)
	if err != nil {
		return err
	}

	for _, finalizer := range remoteKyma.Finalizers {
		controllerutil.RemoveFinalizer(remoteKyma, finalizer)
	}

	err = s.Update(ctx, remoteKyma)
	if err != nil {
		return fmt.Errorf("failed to update remote kyma when removing finalizers: %w", err)
	}
	return nil
}

func (s *SkrContext) DeleteKyma(ctx context.Context) error {
	remoteKyma, err := s.getRemoteKyma(ctx)
	if err != nil {
		return err
	}
	err = s.Delete(ctx, remoteKyma)
	if err != nil {
		return fmt.Errorf("failed to delete remote kyma: %w", err)
	}
	return nil
}

func (s *SkrContext) CreateKymaNamespace(ctx context.Context) error {
	namespace := &apicorev1.Namespace{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: shared.DefaultRemoteNamespace,
			Labels: map[string]string{
				shared.ManagedBy:           shared.ManagedByLabelValue,
				shared.IstioInjectionLabel: shared.EnabledValue,
				shared.WardenLabel:         shared.EnabledValue,
			},
		},
		// setting explicit type meta is required for SSA on Namespaces
		TypeMeta: apimetav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(namespace); err != nil {
		return fmt.Errorf("failed to encode namespace: %w", err)
	}

	patch := client.RawPatch(types.ApplyPatchType, buf.Bytes())
	force := true
	patchOpts := &client.PatchOptions{Force: &force, FieldManager: fieldManager}
	if err := s.Patch(ctx, namespace, patch, patchOpts); err != nil {
		return fmt.Errorf("failed to ensure remote namespace exists: %w", err)
	}

	return nil
}

func (s *SkrContext) CreateOrFetchKyma(
	ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma,
) (*v1beta2.Kyma, error) {
	remoteKyma, err := s.getRemoteKyma(ctx)
	if meta.IsNoMatchError(err) || CRDNotFoundErr(err) {
		if err := s.createOrUpdateCRD(ctx, kcpClient, shared.KymaKind.Plural()); err != nil {
			return nil, err
		}
		s.event.Normal(kyma, crdInstallation, "CRDs were installed to SKR")
	}

	if util.IsNotFound(err) {
		if !kyma.DeletionTimestamp.IsZero() {
			return nil, ErrNotFoundAndKCPKymaUnderDeleting
		}
		kyma.Spec.DeepCopyInto(&remoteKyma.Spec)
		err = s.Create(ctx, remoteKyma)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote kyma: %w", err)
		}
		s.event.Normal(kyma, remoteInstallation, "Kyma was installed to SKR")
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch remote kyma: %w", err)
	}

	return remoteKyma, nil
}

// SynchronizeKymaMetadata synchronizes the metadata to the SKR Kyma CR .
// It sets the required labels and annotations.
func (s *SkrContext) SynchronizeKymaMetadata(ctx context.Context, kcpKyma, skrKyma *v1beta2.Kyma) error {
	if !skrKyma.GetDeletionTimestamp().IsZero() {
		return nil
	}

	watcherLabelsChanged := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	btpRelatedLabelsChanged := syncBTPRelatedLabels(kcpKyma, skrKyma)

	if !watcherLabelsChanged && !btpRelatedLabelsChanged {
		return nil
	}

	metadataToSync := &unstructured.Unstructured{}
	metadataToSync.SetName(skrKyma.GetName())
	metadataToSync.SetNamespace(skrKyma.GetNamespace())
	metadataToSync.SetGroupVersionKind(kcpKyma.GroupVersionKind()) // use KCP GVK as SKR GVK may not be available
	metadataToSync.SetLabels(skrKyma.GetLabels())
	metadataToSync.SetAnnotations(skrKyma.GetAnnotations())

	forceOwnership := true
	err := s.Patch(ctx,
		metadataToSync,
		client.Apply,
		&client.PatchOptions{FieldManager: fieldManager, Force: &forceOwnership})
	if err != nil {
		err = fmt.Errorf("failed to synchronise Kyma metadata to SKR: %w", err)
		s.event.Warning(kcpKyma, metadataSyncFailure, err)
		return err
	}

	return nil
}

// SynchronizeKymaStatus synchronizes the status to the SKR Kyma CR.
func (s *SkrContext) SynchronizeKymaStatus(ctx context.Context, kcpKyma, skrKyma *v1beta2.Kyma) error {
	if !skrKyma.GetDeletionTimestamp().IsZero() {
		return nil
	}

	syncStatus(&kcpKyma.Status, &skrKyma.Status)
	if err := s.Client.Status().Update(ctx, skrKyma); err != nil {
		err = fmt.Errorf("failed to synchronise Kyma status to SKR: %w", err)
		s.event.Warning(kcpKyma, statusSyncFailure, err)
		return err
	}

	return nil
}

// ReplaceSpec replaces 'spec' attributes in control plane Kyma with values from Remote Kyma.
func ReplaceSpec(controlPlaneKyma *v1beta2.Kyma, remoteKyma *v1beta2.Kyma) {
	controlPlaneKyma.Spec.Modules = []v1beta2.Module{}
	controlPlaneKyma.Spec.Modules = append(controlPlaneKyma.Spec.Modules, remoteKyma.Spec.Modules...)
	controlPlaneKyma.Spec.Channel = remoteKyma.Spec.Channel
}

func (s *SkrContext) createOrUpdateCRD(ctx context.Context, kcpClient client.Client, plural string) error {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	crdFromRuntime := &apiextensionsv1.CustomResourceDefinition{}
	var err error
	err = kcpClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
	}, crd,
	)
	if err != nil {
		return fmt.Errorf("failed to get kyma CRDs on kcp: %w", err)
	}

	err = s.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crdFromRuntime,
	)

	if util.IsNotFound(err) || !ContainsLatestVersion(crdFromRuntime, v1beta2.GroupVersion.Version) {
		return PatchCRD(ctx, s.Client, crd)
	}

	if err != nil {
		return fmt.Errorf("failed to get kyma CRDs on remote: %w", err)
	}

	return nil
}

func (s *SkrContext) getRemoteKyma(ctx context.Context) (*v1beta2.Kyma, error) {
	skrKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.DefaultRemoteKymaName,
			Namespace: shared.DefaultRemoteNamespace,
		},
	}

	if err := s.Get(ctx, client.ObjectKeyFromObject(skrKyma), skrKyma); err != nil {
		return skrKyma, fmt.Errorf("failed to get remote kyma: %w", err)
	}

	return skrKyma, nil
}

// syncBTPRelatedLabels sets the BTP related labels on the SKR Kyma CR.
// It returns true if any of the labels were changed.
func syncBTPRelatedLabels(kcpKyma, skrKyma *v1beta2.Kyma) bool {
	labelsMap := map[string]string{}
	globalAccountIDLabelValue, ok := kcpKyma.Labels[shared.GlobalAccountIDLabel]
	if ok {
		labelsMap[shared.GlobalAccountIDLabel] = globalAccountIDLabelValue
	}

	subAccountIDLabelValue, ok := kcpKyma.Labels[shared.SubAccountIDLabel]
	if ok {
		labelsMap[shared.SubAccountIDLabel] = subAccountIDLabelValue
	}

	labels, labelsChanged := collections.MergeMaps(skrKyma.Labels, labelsMap)

	skrKyma.Labels = labels
	return labelsChanged
}

// syncWatcherLabelsAnnotations adds required labels and annotations to the skrKyma.
// It returns true if any of the labels or annotations were changed.
func syncWatcherLabelsAnnotations(kcpKyma, skrKyma *v1beta2.Kyma) bool {
	labels, labelsChanged := collections.MergeMaps(skrKyma.Labels, map[string]string{
		shared.WatchedByLabel: shared.WatchedByLabelValue,
		shared.ManagedBy:      shared.ManagedByLabelValue,
	})
	skrKyma.Labels = labels

	annotations, annotationsChanged := collections.MergeMaps(skrKyma.Annotations, map[string]string{
		shared.OwnedByAnnotation: fmt.Sprintf(shared.OwnedByFormat,
			kcpKyma.GetNamespace(), kcpKyma.GetName()),
	})
	skrKyma.Annotations = annotations

	return labelsChanged || annotationsChanged
}

// syncStatus copies the Kyma status and transofrms it from KCP perspective to SKR perspective.
// E.g., it removes manifest references or changes namespaces.
func syncStatus(kcpStatus, skrStatus *v1beta2.KymaStatus) {
	*skrStatus = *kcpStatus.DeepCopy()

	useRemoteNamespaceForModuleTemplates(skrStatus)
	removeManifestReference(skrStatus)
}

func useRemoteNamespaceForModuleTemplates(status *v1beta2.KymaStatus) {
	for i := range status.Modules {
		if status.Modules[i].Template == nil {
			continue
		}
		status.Modules[i].Template.Namespace = shared.DefaultRemoteNamespace
	}
}

func removeManifestReference(status *v1beta2.KymaStatus) {
	for i := range status.Modules {
		if status.Modules[i].Manifest == nil {
			continue
		}
		status.Modules[i].Manifest = nil
	}
}
