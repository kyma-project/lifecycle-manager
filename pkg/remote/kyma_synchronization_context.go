package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	apicore "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type ClientFunc func() *rest.Config

//nolint:gochecknoglobals
var (
	LocalClient                        ClientFunc
	ErrNotFoundAndKCPKymaUnderDeleting = errors.New("not found and kcp kyma under deleting")
)

type KymaSynchronizationContext struct {
	ControlPlaneClient Client
	RuntimeClient      Client
}

func InitializeKymaSynchronizationContext(ctx context.Context, kcp Client, cache *ClientCache,
	kyma *v1beta2.Kyma, syncNamespace string,
) (*KymaSynchronizationContext, error) {
	strategyValue, found := kyma.Annotations[v1beta2.SyncStrategyAnnotation]
	syncStrategy := v1beta2.SyncStrategyLocalSecret
	if found && strategyValue == v1beta2.SyncStrategyLocalClient {
		syncStrategy = v1beta2.SyncStrategyLocalClient
	}
	skr, err := NewClientLookup(kcp, cache, v1beta2.SyncStrategy(syncStrategy)).
		Lookup(ctx, client.ObjectKeyFromObject(kyma))
	if err != nil {
		return nil, err
	}

	sync := &KymaSynchronizationContext{
		ControlPlaneClient: kcp,
		RuntimeClient:      skr,
	}

	if err := sync.ensureRemoteNamespaceExists(ctx, syncNamespace); err != nil {
		return nil, err
	}

	return sync, nil
}

func (c *KymaSynchronizationContext) GetRemotelySyncedKyma(
	ctx context.Context, remoteSyncNamespace string,
) (*v1beta2.Kyma, error) {
	remoteKyma := &v1beta2.Kyma{}
	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = remoteSyncNamespace
	if err := c.RuntimeClient.Get(ctx, client.ObjectKeyFromObject(remoteKyma), remoteKyma); err != nil {
		return remoteKyma, fmt.Errorf("failed to get remote kyma: %w", err)
	}

	return remoteKyma, nil
}

func RemoveFinalizerFromRemoteKyma(
	ctx context.Context, remoteSyncNamespace string,
) error {
	syncContext, err := SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}

	remoteKyma, err := syncContext.GetRemotelySyncedKyma(ctx, remoteSyncNamespace)
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(remoteKyma, v1beta2.Finalizer)

	err = syncContext.RuntimeClient.Update(ctx, remoteKyma)
	if err != nil {
		return fmt.Errorf("failed to update remote kyma when removing finalizers: %w", err)
	}
	return nil
}

func DeleteRemotelySyncedKyma(
	ctx context.Context, remoteSyncNamespace string,
) error {
	syncContext, err := SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}
	remoteKyma, err := syncContext.GetRemotelySyncedKyma(ctx, remoteSyncNamespace)
	if err != nil {
		return err
	}
	err = syncContext.RuntimeClient.Delete(ctx, remoteKyma)
	if err != nil {
		return fmt.Errorf("failed to delete remote kyma: %w", err)
	}
	return nil
}

// ensureRemoteNamespaceExists tries to ensure existence of a namespace for synchronization based on
// name of controlPlaneKyma.namespace in this order.
func (c *KymaSynchronizationContext) ensureRemoteNamespaceExists(ctx context.Context, syncNamespace string) error {
	namespace := &apicore.Namespace{
		ObjectMeta: apimachinerymeta.ObjectMeta{
			Name:   syncNamespace,
			Labels: map[string]string{v1beta2.ManagedBy: v1beta2.OperatorName},
		},
		// setting explicit type meta is required for SSA on Namespaces
		TypeMeta: apimachinerymeta.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(namespace); err != nil {
		return fmt.Errorf("failed to encode namespace: %w", err)
	}

	patch := client.RawPatch(types.ApplyPatchType, buf.Bytes())
	force := true
	fieldManager := "kyma-sync-context"

	if err := c.RuntimeClient.Patch(
		ctx, namespace, patch, &client.PatchOptions{Force: &force, FieldManager: fieldManager},
	); err != nil {
		return fmt.Errorf("failed to ensure remote namespace exists: %w", err)
	}

	return nil
}

func (c *KymaSynchronizationContext) CreateOrUpdateCRD(ctx context.Context, plural string) error {
	crd := &apiextensions.CustomResourceDefinition{}
	crdFromRuntime := &apiextensions.CustomResourceDefinition{}
	var err error
	err = c.ControlPlaneClient.Get(
		ctx, client.ObjectKey{
			// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
			// name changes, this also has to be adjusted here. We can think of making this configurable later
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crd,
	)

	if err != nil {
		return fmt.Errorf("failed to get kyma CRDs on kcp: %w", err)
	}

	err = c.RuntimeClient.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crdFromRuntime,
	)

	if util.IsNotFound(err) || !ContainsLatestVersion(crdFromRuntime, v1beta2.GroupVersion.Version) {
		return PatchCRD(ctx, c.RuntimeClient, crd)
	}

	if err != nil {
		return fmt.Errorf("failed to get kyma CRDs on remote: %w", err)
	}

	return nil
}

func (c *KymaSynchronizationContext) CreateOrFetchRemoteKyma(
	ctx context.Context, kyma *v1beta2.Kyma, remoteSyncNamespace string,
) (*v1beta2.Kyma, error) {
	recorder := adapter.RecorderFromContext(ctx)

	remoteKyma, err := c.GetRemotelySyncedKyma(ctx, remoteSyncNamespace)
	if meta.IsNoMatchError(err) || CRDNotFoundErr(err) {
		recorder.Event(kyma, "Normal", err.Error(), "CRDs are missing in SKR and will be installed")

		if err := c.CreateOrUpdateCRD(ctx, v1beta2.KymaKind.Plural()); err != nil {
			return nil, err
		}

		recorder.Event(kyma, "Normal", "CRDInstallation", "CRDs were installed to SKR")
	}

	if util.IsNotFound(err) {
		if !kyma.DeletionTimestamp.IsZero() {
			return nil, ErrNotFoundAndKCPKymaUnderDeleting
		}
		kyma.Spec.DeepCopyInto(&remoteKyma.Spec)

		err = c.RuntimeClient.Create(ctx, remoteKyma)
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

func (c *KymaSynchronizationContext) SynchronizeRemoteKyma(
	ctx context.Context,
	controlPlaneKyma, remoteKyma *v1beta2.Kyma,
) error {
	if !remoteKyma.GetDeletionTimestamp().IsZero() {
		return nil
	}
	recorder := adapter.RecorderFromContext(ctx)

	c.SyncWatcherLabelsAnnotations(controlPlaneKyma, remoteKyma)
	if err := c.RuntimeClient.Update(ctx, remoteKyma); err != nil {
		recorder.Event(controlPlaneKyma, "Warning", err.Error(), "could not synchronise runtime kyma "+
			"spec, watcher labels and annotations")
		return fmt.Errorf("failed to synchronise runtime kyma: %w", err)
	}

	remoteKyma.Status = controlPlaneKyma.Status
	if err := c.RuntimeClient.Status().Update(ctx, remoteKyma); err != nil {
		recorder.Event(controlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma status")
		return fmt.Errorf("failed to update runtime kyma status: %w", err)
	}
	return nil
}

// ReplaceModules replaces modules specification from control plane Kyma with Remote Kyma specifications.
func ReplaceModules(
	controlPlaneKyma *v1beta2.Kyma,
	remoteKyma *v1beta2.Kyma,
) {
	controlPlaneKyma.Spec.Modules = []v1beta2.Module{}
	controlPlaneKyma.Spec.Modules = append(controlPlaneKyma.Spec.Modules, remoteKyma.Spec.Modules...)
	controlPlaneKyma.Spec.Channel = remoteKyma.Spec.Channel
}

// SyncWatcherLabelsAnnotations inserts labels into the given KymaCR, which are needed to ensure
// a working e2e-flow for the runtime-watcher.
func (c *KymaSynchronizationContext) SyncWatcherLabelsAnnotations(controlPlaneKyma, remoteKyma *v1beta2.Kyma) {
	if remoteKyma.Labels == nil {
		remoteKyma.Labels = make(map[string]string)
	}

	remoteKyma.Labels[v1beta2.WatchedByLabel] = v1beta2.OperatorName

	if remoteKyma.Annotations == nil {
		remoteKyma.Annotations = make(map[string]string)
	}
	remoteKyma.Annotations[v1beta2.OwnedByAnnotation] = fmt.Sprintf(v1beta2.OwnedByFormat,
		controlPlaneKyma.GetNamespace(), controlPlaneKyma.GetName())
}
