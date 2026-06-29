package remote

import (
	"context"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apiconfigsv1beta2 "github.com/kyma-project/lifecycle-manager/api/applyconfigurations/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrNotFoundAndKCPKymaUnderDeleting = errors.New("not found and kcp kyma under deleting")

const (
	remoteInstallation  event.Reason = "RemoteInstallation"
	metadataSyncFailure event.Reason = "MetadataSynchronization"
	statusSyncFailure   event.Reason = "StatusSynchronization"
)

type SkrContext struct {
	client.Client

	event event.Event
}

func NewSkrContext(client client.Client, event event.Event) *SkrContext {
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
	namespace := corev1apply.Namespace(shared.DefaultRemoteNamespace).
		WithLabels(map[string]string{
			shared.ManagedBy:           shared.ManagedByLabelValue,
			shared.IstioInjectionLabel: shared.EnabledValue,
			shared.WardenLabel:         shared.EnabledValue,
		})

	if err := s.Apply(ctx, namespace, client.ForceOwnership, fieldowners.KymaSyncContextProvider); err != nil {
		return fmt.Errorf("failed to ensure remote namespace exists: %w", err)
	}

	return nil
}

func (s *SkrContext) CreateOrFetchKyma(
	ctx context.Context, kyma *v1beta2.Kyma,
) (*v1beta2.Kyma, error) {
	remoteKyma, err := s.getRemoteKyma(ctx)
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

	applyConfig := apiconfigsv1beta2.Kyma(skrKyma.GetName(), skrKyma.GetNamespace()).
		WithLabels(skrKyma.GetLabels()).
		WithAnnotations(skrKyma.GetAnnotations())

	if err := s.Apply(ctx, applyConfig, client.ForceOwnership, fieldowners.KymaSyncContextProvider); err != nil {
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
	targetLabels := map[string]string{
		shared.WatchedByLabel: shared.WatchedByLabelValue,
		shared.ManagedBy:      shared.ManagedByLabelValue,
	}
	runtimeIDLabelValue, ok := kcpKyma.Labels[shared.RuntimeIDLabel]
	if ok {
		targetLabels[shared.RuntimeIDLabel] = runtimeIDLabelValue
	}

	labels, labelsChanged := collections.MergeMaps(skrKyma.Labels, targetLabels)
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
		status.Modules[i].Template.PartialMeta.Namespace = shared.DefaultRemoteNamespace
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
