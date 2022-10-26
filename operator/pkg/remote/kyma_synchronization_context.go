package remote

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/adapter"
)

type ClientFunc func() *rest.Config

var (
	LocalClient             ClientFunc //nolint:gochecknoglobals
	ErrNoLocalClientDefined = errors.New("no local client defined")
)

type KymaSynchronizationContext struct {
	ControlPlaneClient   client.Client
	RuntimeClient        client.Client
	ControlPlaneKyma     *v1alpha1.Kyma
	statusUpdateRequired bool
}

func (c *KymaSynchronizationContext) RequiresStatusUpdateInControlPlane() bool {
	return c.statusUpdateRequired
}

func (c *KymaSynchronizationContext) RequireStatusUpdateInControlPlane() {
	c.statusUpdateRequired = true
}

func NewRemoteClient(ctx context.Context, controlPlaneClient client.Client, key client.ObjectKey,
	strategy v1alpha1.SyncStrategy, cache *ClientCache,
) (client.Client, error) {
	remoteClient := cache.Get(ClientCacheID(key))

	if remoteClient != nil {
		return remoteClient, nil
	}

	clusterClient := ClusterClient{
		DefaultClient: controlPlaneClient,
		Logger:        log.FromContext(ctx),
	}

	var restConfig *rest.Config

	var err error

	switch strategy {
	case v1alpha1.SyncStrategyLocalClient:
		if LocalClient != nil {
			restConfig = LocalClient()
		} else {
			err = ErrNoLocalClientDefined
		}
	case v1alpha1.SyncStrategyLocalSecret:
		fallthrough
	default:
		restConfig, err = clusterClient.GetRestConfigFromSecret(ctx, key.Name, key.Namespace)
	}

	if err != nil {
		return nil, err
	}

	remoteClient, err = client.New(restConfig, client.Options{Scheme: controlPlaneClient.Scheme()})
	if err != nil {
		return nil, err
	}
	cache.Set(ClientCacheID(key), remoteClient)

	return remoteClient, nil
}

func GetRemotelySyncedKyma(ctx context.Context, runtimeClient client.Client,
	key client.ObjectKey,
) (*v1alpha1.Kyma, error) {
	remoteKyma := &v1alpha1.Kyma{}
	if err := runtimeClient.Get(ctx, key, remoteKyma); err != nil {
		return nil, err
	}

	return remoteKyma, nil
}

func DeleteRemotelySyncedKyma(
	ctx context.Context, controlPlaneClient client.Client, cache *ClientCache, kyma *v1alpha1.Kyma,
) error {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, cache)
	if err != nil {
		return err
	}

	remoteKyma, err := GetRemotelySyncedKyma(ctx, runtimeClient, GetRemoteObjectKey(kyma))
	if err != nil {
		return err
	}

	return runtimeClient.Delete(ctx, remoteKyma)
}

func RemoveFinalizerFromRemoteKyma(
	ctx context.Context, kyma *v1alpha1.Kyma, syncContext *KymaSynchronizationContext,
) error {
	remoteKyma, err := GetRemotelySyncedKyma(ctx, syncContext.RuntimeClient, GetRemoteObjectKey(kyma))
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(remoteKyma, v1alpha1.Finalizer)

	return syncContext.RuntimeClient.Update(ctx, remoteKyma)
}

func InitializeKymaSynchronizationContext(ctx context.Context, controlPlaneClient client.Client,
	controlPlaneKyma *v1alpha1.Kyma, cache *ClientCache,
) (*KymaSynchronizationContext, error) {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, client.ObjectKeyFromObject(controlPlaneKyma),
		controlPlaneKyma.Spec.Sync.Strategy, cache)
	if err != nil {
		return nil, err
	}

	sync := &KymaSynchronizationContext{
		ControlPlaneClient: controlPlaneClient,
		RuntimeClient:      runtimeClient,
		ControlPlaneKyma:   controlPlaneKyma,
	}

	return sync, nil
}

func (c *KymaSynchronizationContext) CreateOrUpdateCRD(ctx context.Context, plural string) error {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}
	var err error
	err = c.ControlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crd)

	if err != nil {
		return err
	}

	err = c.RuntimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crdFromRuntime)

	if k8serrors.IsNotFound(err) {
		return c.RuntimeClient.Create(ctx, &v1extensions.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: crd.Name, Namespace: crd.Namespace}, Spec: crd.Spec,
		})
	}

	if err != nil {
		return err
	}

	// crd.SetResourceVersion(crdFromRuntime.GetResourceVersion())
	// return c.runtimeClient.Update(ctx, &v1extensions.CustomResourceDefinition{
	// 	ObjectMeta: v1.ObjectMeta{Name: crd.Name, Namespace: crd.Namespace}, Spec: crd.Spec,
	// })
	return nil
}

func (c *KymaSynchronizationContext) CreateOrFetchRemoteKyma(ctx context.Context) (*v1alpha1.Kyma, error) {
	kyma := c.ControlPlaneKyma
	recorder := adapter.RecorderFromContext(ctx)
	remoteKyma := &v1alpha1.Kyma{}

	remoteKyma.Name = kyma.Name
	remoteKyma.Namespace = kyma.Namespace
	if c.ControlPlaneKyma.Spec.Sync.Namespace != "" {
		remoteKyma.Namespace = c.ControlPlaneKyma.Spec.Sync.Namespace
	}

	err := c.RuntimeClient.Get(ctx, client.ObjectKeyFromObject(remoteKyma), remoteKyma)

	if meta.IsNoMatchError(err) {
		recorder.Event(kyma, "Normal", err.Error(), "CRDs are missing in SKR and will be installed")

		if err := c.CreateOrUpdateCRD(ctx, v1alpha1.KymaKind.Plural()); err != nil {
			return nil, err
		}

		recorder.Event(kyma, "Normal", "CRDInstallation", "CRDs were installed to SKR")
		// the NoMatch error we previously encountered is now fixed through the CRD installation
		err = nil
	}

	if k8serrors.IsNotFound(err) {
		if err := c.EnsureNamespaceExists(ctx, remoteKyma.Namespace); err != nil {
			recorder.Event(kyma, "Warning", "RemoteKymaInstallation",
				fmt.Sprintf("namespace %s could not be synced", remoteKyma.Namespace))

			return nil, err
		}

		kyma.Spec.DeepCopyInto(&remoteKyma.Spec)

		if kyma.Spec.Sync.NoModuleCopy {
			remoteKyma.Spec.Modules = []v1alpha1.Module{}
		}

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

func (c *KymaSynchronizationContext) SynchronizeRemoteKyma(ctx context.Context,
	remoteKyma *v1alpha1.Kyma,
) error {
	recorder := adapter.RecorderFromContext(ctx)

	remoteKyma.Status = c.ControlPlaneKyma.Status

	if err := c.RuntimeClient.Status().Update(ctx, remoteKyma); err != nil {
		recorder.Event(c.ControlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma status")
		return err
	}

	c.InsertWatcherLabels(remoteKyma)

	if err := c.RuntimeClient.Update(ctx, remoteKyma.SetLastSync()); err != nil {
		recorder.Event(c.ControlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma last sync annotation")
		return err
	}

	return nil
}

// ReplaceWithVirtualKyma creates a virtual kyma instance from a control plane Kyma and N Remote Kymas,
// merging the module specification in the process.
func (c *KymaSynchronizationContext) ReplaceWithVirtualKyma(kyma *v1alpha1.Kyma,
	remotes ...*v1alpha1.Kyma,
) {
	totalModuleAmount := len(kyma.Spec.Modules)
	for _, remote := range remotes {
		totalModuleAmount += len(remote.Spec.Modules)
	}
	modules := make(map[string]v1alpha1.Module, totalModuleAmount)

	for _, remote := range remotes {
		for _, m := range remote.Spec.Modules {
			modules[m.Name] = m
		}
	}
	for _, m := range kyma.Spec.Modules {
		modules[m.Name] = m
	}

	kyma.Spec.Modules = []v1alpha1.Module{}
	for _, m := range modules {
		kyma.Spec.Modules = append(kyma.Spec.Modules, m)
	}
}

func (c *KymaSynchronizationContext) EnsureNamespaceExists(ctx context.Context, namespace string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	var err error
	if err = c.RuntimeClient.Get(ctx, client.ObjectKey{Name: namespace}, ns); k8serrors.IsNotFound(err) {
		return c.RuntimeClient.Create(ctx, ns)
	}
	return err
}

func GetRemoteObjectKey(kyma *v1alpha1.Kyma) client.ObjectKey {
	name := kyma.Name
	namespace := kyma.Namespace
	if kyma.Spec.Sync.Namespace != "" {
		namespace = kyma.Spec.Sync.Namespace
	}
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// InsertWatcherLabels inserts labels into the given KymaCR, which are needed to ensure
// a working e2e-flow for the runtime-watcher.
func (c *KymaSynchronizationContext) InsertWatcherLabels(remoteKyma *v1alpha1.Kyma) {
	if remoteKyma.Labels == nil {
		remoteKyma.Labels = make(map[string]string)
	}

	remoteKyma.Labels[v1alpha1.OwnedByLabel] = fmt.Sprintf(v1alpha1.OwnedByFormat,
		c.ControlPlaneKyma.Namespace, c.ControlPlaneKyma.Name)
	remoteKyma.Labels[v1alpha1.WatchedByLabel] = v1alpha1.OperatorName
}
