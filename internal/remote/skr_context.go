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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrNotFoundAndKCPKymaUnderDeleting = errors.New("not found and kcp kyma under deleting")

const (
	crdInstallation     event.Reason = "CRDInstallation"
	remoteInstallation  event.Reason = "RemoteInstallation"
	remoteUpdateFailure event.Reason = "RemoteSynchronization"
	statusUpdateFailure event.Reason = "UpdateRuntimeStatus"
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

	err = s.Client.Update(ctx, remoteKyma)
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
	err = s.Client.Delete(ctx, remoteKyma)
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
				shared.ManagedBy:           shared.KymaLabelValue,
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
	patchOpts := &client.PatchOptions{Force: &force, FieldManager: "kyma-sync-context"}
	if err := s.Client.Patch(ctx, namespace, patch, patchOpts); err != nil {
		return fmt.Errorf("failed to ensure remote namespace exists: %w", err)
	}

	return nil
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

	err = s.Client.Get(
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
		err = s.Client.Create(ctx, remoteKyma)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote kyma: %w", err)
		}
		s.event.Normal(kyma, remoteInstallation, "Kyma was installed to SKR")
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch remote kyma: %w", err)
	}

	return remoteKyma, nil
}

func (s *SkrContext) SynchronizeKyma(ctx context.Context, kcpKyma, remoteKyma *v1beta2.Kyma) error {
	if !remoteKyma.GetDeletionTimestamp().IsZero() {
		return nil
	}

	s.syncWatcherLabelsAnnotations(kcpKyma, remoteKyma)
	if err := s.Client.Update(ctx, remoteKyma); err != nil {
		err = fmt.Errorf("failed to synchronise runtime kyma: %w", err)
		s.event.Warning(kcpKyma, remoteUpdateFailure, err)
		return err
	}

	remoteKyma.Status = kcpKyma.Status
	if err := s.Client.Status().Update(ctx, remoteKyma); err != nil {
		err = fmt.Errorf("failed to update runtime kyma status: %w", err)
		s.event.Warning(kcpKyma, statusUpdateFailure, err)
		return err
	}
	return nil
}

// ReplaceModules replaces modules specification from control plane Kyma with Remote Kyma specifications.
func ReplaceModules(controlPlaneKyma *v1beta2.Kyma, remoteKyma *v1beta2.Kyma) {
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

	if err := s.Client.Get(ctx, client.ObjectKeyFromObject(skrKyma), skrKyma); err != nil {
		return skrKyma, fmt.Errorf("failed to get remote kyma: %w", err)
	}

	return skrKyma, nil
}

// syncWatcherLabelsAnnotations inserts labels into the given KymaCR, which are needed to ensure
// a working e2e-flow for the runtime-watcher.
func (s *SkrContext) syncWatcherLabelsAnnotations(controlPlaneKyma, remoteKyma *v1beta2.Kyma) {
	if remoteKyma.Labels == nil {
		remoteKyma.Labels = make(map[string]string)
	}

	remoteKyma.Labels[shared.WatchedByLabel] = shared.OperatorName

	if remoteKyma.Annotations == nil {
		remoteKyma.Annotations = make(map[string]string)
	}
	remoteKyma.Annotations[shared.OwnedByAnnotation] = fmt.Sprintf(shared.OwnedByFormat,
		controlPlaneKyma.GetNamespace(), controlPlaneKyma.GetName())
}
