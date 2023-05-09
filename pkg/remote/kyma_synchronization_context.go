package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
)

type ClientFunc func() *rest.Config

var (
	LocalClient                        ClientFunc //nolint:gochecknoglobals
	ErrNoLocalClientDefined            = errors.New("no local client defined")
	ErrNotFoundAndKCPKymaUnderDeleting = errors.New("not found and kcp kyma under deleting")
)

type KymaSynchronizationContext struct {
	ControlPlaneClient Client
	RuntimeClient      Client
}

func InitializeKymaSynchronizationContext(
	ctx context.Context, kcp Client, cache *ClientCache, kyma *v1beta2.Kyma, syncNamespace string,
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
	ctx context.Context, controlPlaneKyma *v1beta2.Kyma, remoteSyncNamespace string,
) (*v1beta2.Kyma, error) {
	remoteKyma := &v1beta2.Kyma{}
	if err := c.RuntimeClient.Get(ctx, GetRemoteObjectKey(controlPlaneKyma, remoteSyncNamespace), remoteKyma); err != nil {
		return nil, err
	}

	return remoteKyma, nil
}

func RemoveFinalizerFromRemoteKyma(
	ctx context.Context, kyma *v1beta2.Kyma, remoteSyncNamespace string,
) error {
	syncContext := SyncContextFromContext(ctx)

	remoteKyma, err := syncContext.GetRemotelySyncedKyma(ctx, kyma, remoteSyncNamespace)
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(remoteKyma, v1beta2.Finalizer)

	return syncContext.RuntimeClient.Update(ctx, remoteKyma)
}

func DeleteRemotelySyncedKyma(
	ctx context.Context, kyma *v1beta2.Kyma, remoteSyncNamespace string,
) error {
	syncContext := SyncContextFromContext(ctx)
	remoteKyma, err := syncContext.GetRemotelySyncedKyma(ctx, kyma, remoteSyncNamespace)
	if err != nil {
		return err
	}

	return syncContext.RuntimeClient.Delete(ctx, remoteKyma)
}

// ensureRemoteNamespaceExists tries to ensure existence of a namespace for synchronization based on
// name of controlPlaneKyma.namespace in this order.
func (c *KymaSynchronizationContext) ensureRemoteNamespaceExists(ctx context.Context, syncNamespace string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   syncNamespace,
			Labels: map[string]string{v1beta2.ManagedBy: v1beta2.OperatorName},
		},
		// setting explicit type meta is required for SSA on Namespaces
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(namespace); err != nil {
		return err
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

func UpdateKymaAnnotations(
	kyma *v1beta2.Kyma,
	kcpCRD *v1extensions.CustomResourceDefinition,
	skrCRD *v1extensions.CustomResourceDefinition) {
	if kyma.Annotations == nil {
		kyma.Annotations = make(map[string]string)
	}

	kcpAnnotation := v1beta2.KcpKymaCRDGenerationAnnotation
	skrAnnotation := v1beta2.SkrKymaCRDGenerationAnnotation

	if kcpCRD.Spec.Names.Kind == string(v1beta2.ModuleTemplateKind) {
		kcpAnnotation = v1beta2.KcpModuleTemplateCRDGenerationAnnotation
		skrAnnotation = v1beta2.SkrModuleTemplateCRDGenerationAnnotation
	}

	kyma.Annotations[kcpAnnotation] = strconv.FormatInt(kcpCRD.Generation, 10)
	kyma.Annotations[skrAnnotation] = strconv.FormatInt(skrCRD.Generation, 10)
}

func CreateOrUpdateCRD(
	ctx context.Context, plural string, kyma *v1beta2.Kyma, runtimeClient Client, controlPlaneClient Client) (
	*v1extensions.CustomResourceDefinition, *v1extensions.CustomResourceDefinition, error) {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}
	var err error
	err = controlPlaneClient.Get(
		ctx, client.ObjectKey{
			// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
			// name changes, this also has to be adjusted here. We can think of making this configurable later
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crd,
	)

	if err != nil {
		return nil, nil, err
	}

	err = runtimeClient.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crdFromRuntime,
	)

	kcpAnnotation := v1beta2.KcpKymaCRDGenerationAnnotation
	skrAnnotation := v1beta2.SkrKymaCRDGenerationAnnotation

	if plural == v1beta2.ModuleTemplateKind.Plural() {
		kcpAnnotation = v1beta2.KcpModuleTemplateCRDGenerationAnnotation
		skrAnnotation = v1beta2.SkrModuleTemplateCRDGenerationAnnotation
	}

	latestGeneration := strconv.FormatInt(crd.Generation, 10)
	runtimeCRDGeneration := strconv.FormatInt(crdFromRuntime.Generation, 10)
	if k8serrors.IsNotFound(err) || !ContainsLatestVersion(crdFromRuntime, v1beta2.GroupVersion.Version) ||
		!ContainsLatestCRDGeneration(kyma.Annotations[kcpAnnotation], latestGeneration) ||
		!ContainsLatestCRDGeneration(kyma.Annotations[skrAnnotation], runtimeCRDGeneration) {
		err = PatchCRD(ctx, runtimeClient, crd)
		if err != nil {
			return nil, nil, err
		}

		err = runtimeClient.Get(
			ctx, client.ObjectKey{
				Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
			}, crdFromRuntime,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	if plural == v1beta2.ModuleTemplateKind.Plural() && !crdReady(crdFromRuntime) {
		return nil, nil, ErrTemplateCRDNotReady
	}

	if err != nil {
		return nil, nil, err
	}

	return crd, crdFromRuntime, nil
}

func (c *KymaSynchronizationContext) CreateOrFetchRemoteKyma(
	ctx context.Context, kyma *v1beta2.Kyma, remoteSyncNamespace string,
) (*v1beta2.Kyma, error) {
	recorder := adapter.RecorderFromContext(ctx)
	remoteKyma := &v1beta2.Kyma{}

	remoteKyma.Name = kyma.Name
	remoteKyma.Namespace = remoteSyncNamespace

	err := c.RuntimeClient.Get(ctx, client.ObjectKeyFromObject(remoteKyma), remoteKyma)

	if meta.IsNoMatchError(err) || err == nil {
		if meta.IsNoMatchError(err) {
			recorder.Event(kyma, "Normal", err.Error(), "CRDs are missing in SKR and will be installed")
		}
		var kcpCrd, skrCrd *v1extensions.CustomResourceDefinition
		if kcpCrd, skrCrd, err = CreateOrUpdateCRD(
			ctx, v1beta2.KymaKind.Plural(), kyma, c.RuntimeClient, c.ControlPlaneClient); err != nil {
			return nil, err
		}

		if !ContainsLatestCRDGeneration(kyma.Annotations[v1beta2.KcpKymaCRDGenerationAnnotation], strconv.FormatInt(kcpCrd.Generation, 10)) ||
			!ContainsLatestCRDGeneration(kyma.Annotations[v1beta2.SkrKymaCRDGenerationAnnotation], strconv.FormatInt(skrCrd.Generation, 10)) {
			UpdateKymaAnnotations(kyma, kcpCrd, skrCrd)
			if err = c.ControlPlaneClient.Update(ctx, kyma); err != nil {
				recorder.Event(kyma, "Warning", err.Error(), "Couldn't update Kyma CR with CRD generations.")
			}
		}
		recorder.Event(kyma, "Normal", "CRDInstallation", "CRDs were installed to SKR")
		// the NoMatch error we previously encountered is now fixed through the CRD installation
		err = nil
	}

	if k8serrors.IsNotFound(err) {
		if !kyma.DeletionTimestamp.IsZero() {
			return nil, ErrNotFoundAndKCPKymaUnderDeleting
		}
		kyma.Spec.DeepCopyInto(&remoteKyma.Spec)

		// if KCP Kyma contains some modules during initialization, not sync them into remote.
		remoteKyma.Spec.Modules = []v1beta2.Module{}

		err = c.RuntimeClient.Create(ctx, remoteKyma)
		if err != nil {
			recorder.Event(kyma, "Normal", "RemoteInstallation", "Kyma was installed to SKR")

			return nil, err
		}
	} else if err != nil {
		recorder.Event(kyma, "Warning", err.Error(), "Client could not fetch remote Kyma")

		return nil, err
	}

	return remoteKyma, err
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
		return err
	}

	remoteKyma.Status = controlPlaneKyma.Status
	if err := c.RuntimeClient.Status().Update(ctx, remoteKyma); err != nil {
		recorder.Event(controlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma status")
		return err
	}

	return nil
}

// ReplaceWithVirtualKyma creates a virtual kyma instance from a control plane Kyma and N Remote Kymas,
// merging the module specification in the process.
func (c *KymaSynchronizationContext) ReplaceWithVirtualKyma(
	kyma *v1beta2.Kyma,
	remotes ...*v1beta2.Kyma,
) {
	totalModuleAmount := len(kyma.Spec.Modules)
	for _, remote := range remotes {
		totalModuleAmount += len(remote.Spec.Modules)
	}
	modules := make(map[string]v1beta2.Module, totalModuleAmount)

	for _, remote := range remotes {
		for _, m := range remote.Spec.Modules {
			modules[m.Name] = m
		}
	}
	for _, m := range kyma.Spec.Modules {
		modules[m.Name] = m
	}

	kyma.Spec.Modules = []v1beta2.Module{}
	for _, m := range modules {
		kyma.Spec.Modules = append(kyma.Spec.Modules, m)
	}
}

func GetRemoteObjectKey(kyma *v1beta2.Kyma, remoteSyncNamespace string) client.ObjectKey {
	name := kyma.Name
	namespace := remoteSyncNamespace
	return client.ObjectKey{Namespace: namespace, Name: name}
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
