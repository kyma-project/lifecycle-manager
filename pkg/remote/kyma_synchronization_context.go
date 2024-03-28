package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrNotFoundAndKCPKymaUnderDeleting = errors.New("not found and kcp kyma under deleting")

type KymaClientFactory interface {
	GetClient(ctx context.Context, kyma *v1beta2.Kyma) (*KymaClient, error)
}

type KymaClient struct {
	Client
}

type KymaSyncContextFactory struct {
	clientCache  *ClientCache
	clientLookup *ClientLookup
}

func (k *KymaSyncContextFactory) GetClient(ctx context.Context, kyma *v1beta2.Kyma) (*KymaClient, error) {
	skrClient, err := k.clientLookup.Lookup(ctx, client.ObjectKeyFromObject(kyma))
	if err != nil {
		return nil, err
	}

	syncContext := &KymaClient{
		Client: skrClient,
	}

	if err := syncContext.ensureNamespaceExists(ctx); err != nil {
		return nil, err
	}

	return syncContext, nil
}

func (k *KymaClient) getRemotelySyncedKyma(ctx context.Context) (*v1beta2.Kyma, error) {
	skrKyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      shared.DefaultRemoteKymaName,
			Namespace: shared.DefaultRemoteNamespace,
		},
	}

	if err := k.Client.Get(ctx, client.ObjectKeyFromObject(skrKyma), skrKyma); err != nil {
		return skrKyma, fmt.Errorf("failed to get remote kyma: %w", err)
	}

	return skrKyma, nil
}

func (k *KymaClient) RemoveFinalizersFromRemoteKyma(ctx context.Context) error {
	remoteKyma, err := k.getRemotelySyncedKyma(ctx)
	if err != nil {
		return err
	}

	for _, finalizer := range remoteKyma.Finalizers {
		controllerutil.RemoveFinalizer(remoteKyma, finalizer)
	}

	err = k.Client.Update(ctx, remoteKyma)
	if err != nil {
		return fmt.Errorf("failed to update remote kyma when removing finalizers: %w", err)
	}
	return nil
}

func (k *KymaClient) DeleteRemotelySyncedKyma(ctx context.Context) error {
	remoteKyma, err := k.getRemotelySyncedKyma(ctx)
	if err != nil {
		return err
	}
	err = k.Client.Delete(ctx, remoteKyma)
	if err != nil {
		return fmt.Errorf("failed to delete remote kyma: %w", err)
	}
	return nil
}

func (k *KymaClient) ensureNamespaceExists(ctx context.Context) error {
	namespace := &apicorev1.Namespace{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:   shared.DefaultRemoteNamespace,
			Labels: map[string]string{shared.ManagedBy: shared.OperatorName},
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
	if err := k.Client.Patch(ctx, namespace, patch, patchOpts); err != nil {
		return fmt.Errorf("failed to ensure remote namespace exists: %w", err)
	}

	return nil
}

func (k *KymaClient) createOrUpdateCRD(ctx context.Context, plural string) error {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	crdFromRuntime := &apiextensionsv1.CustomResourceDefinition{}
	var err error
	err = k.Client.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
	}, crd,
	)
	if err != nil {
		return fmt.Errorf("failed to get kyma CRDs on kcp: %w", err)
	}

	err = k.Client.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crdFromRuntime,
	)

	if util.IsNotFound(err) || !ContainsLatestVersion(crdFromRuntime, v1beta2.GroupVersion.Version) {
		return PatchCRD(ctx, k.Client, crd)
	}

	if err != nil {
		return fmt.Errorf("failed to get kyma CRDs on remote: %w", err)
	}

	return nil
}

func (k *KymaClient) CreateOrFetchRemoteKyma(ctx context.Context, kyma *v1beta2.Kyma) (*v1beta2.Kyma, error) {
	recorder := adapter.RecorderFromContext(ctx)
	remoteKyma, err := k.getRemotelySyncedKyma(ctx)
	if meta.IsNoMatchError(err) || CRDNotFoundErr(err) {
		recorder.Event(kyma, "Normal", err.Error(), "CRDs are missing in SKR and will be installed")

		if err := k.createOrUpdateCRD(ctx, shared.KymaKind.Plural()); err != nil {
			return nil, err
		}

		recorder.Event(kyma, "Normal", "CRDInstallation", "CRDs were installed to SKR")
	}

	if util.IsNotFound(err) {
		if !kyma.DeletionTimestamp.IsZero() {
			return nil, ErrNotFoundAndKCPKymaUnderDeleting
		}
		kyma.Spec.DeepCopyInto(&remoteKyma.Spec)

		err = k.Client.Create(ctx, remoteKyma)
		if err != nil {
			recorder.Event(kyma, "Normal", "RemoteInstallation", "Kyma was installed to SKR")
			return nil, fmt.Errorf("failed to create remote kyma: %w", err)
		}
	} else if err != nil {
		recorder.Event(kyma, "Warning", err.Error(), "Client could not fetch remote Kyma")
		return nil, fmt.Errorf("failed to fetch remote kyma: %w", err)
	}

	return remoteKyma, nil
}

func (k *KymaClient) SynchronizeRemoteKyma(ctx context.Context, controlPlaneKyma, remoteKyma *v1beta2.Kyma) error {
	if !remoteKyma.GetDeletionTimestamp().IsZero() {
		return nil
	}
	recorder := adapter.RecorderFromContext(ctx)

	k.syncWatcherLabelsAnnotations(controlPlaneKyma, remoteKyma)
	if err := k.Client.Update(ctx, remoteKyma); err != nil {
		recorder.Event(controlPlaneKyma, "Warning", err.Error(), "could not synchronise runtime kyma "+
			"spec, watcher labels and annotations")
		return fmt.Errorf("failed to synchronise runtime kyma: %w", err)
	}

	remoteKyma.Status = controlPlaneKyma.Status
	if err := k.Client.Status().Update(ctx, remoteKyma); err != nil {
		recorder.Event(controlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma status")
		return fmt.Errorf("failed to update runtime kyma status: %w", err)
	}
	return nil
}

// ReplaceModules replaces modules specification from control plane Kyma with Remote Kyma specifications.
func ReplaceModules(controlPlaneKyma *v1beta2.Kyma, remoteKyma *v1beta2.Kyma) {
	controlPlaneKyma.Spec.Modules = []v1beta2.Module{}
	controlPlaneKyma.Spec.Modules = append(controlPlaneKyma.Spec.Modules, remoteKyma.Spec.Modules...)
	controlPlaneKyma.Spec.Channel = remoteKyma.Spec.Channel
}

// syncWatcherLabelsAnnotations inserts labels into the given KymaCR, which are needed to ensure
// a working e2e-flow for the runtime-watcher.
func (k *KymaClient) syncWatcherLabelsAnnotations(controlPlaneKyma, remoteKyma *v1beta2.Kyma) {
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
