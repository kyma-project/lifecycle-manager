package remote

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
)

type ClientFunc func() *rest.Config

var (
	LocalClient             ClientFunc //nolint:gochecknoglobals
	ErrNoLocalClientDefined = errors.New("no local client defined")
)

type KymaSynchronizationContext struct {
	controlPlaneClient client.Client
	runtimeClient      client.Client
	controlPlaneKyma   *operatorv1alpha1.Kyma
}

func NewRemoteClient(ctx context.Context, controlPlaneClient client.Client, key client.ObjectKey,
	strategy operatorv1alpha1.SyncStrategy,
) (client.Client, error) {
	clusterClient := ClusterClient{
		DefaultClient: controlPlaneClient,
		Logger:        log.FromContext(ctx),
	}

	var restConfig *rest.Config

	var err error

	switch strategy {
	case operatorv1alpha1.SyncStrategyLocalClient:
		if LocalClient != nil {
			restConfig = LocalClient()
		} else {
			err = ErrNoLocalClientDefined
		}
	case operatorv1alpha1.SyncStrategyLocalSecret:
		fallthrough
	default:
		restConfig, err = clusterClient.GetRestConfigFromSecret(ctx, key.Name, key.Namespace)
	}

	if err != nil {
		return nil, err
	}

	remoteClient, err := client.New(restConfig, client.Options{Scheme: controlPlaneClient.Scheme()})
	if err != nil {
		return nil, err
	}

	return remoteClient, nil
}

func GetRemotelySyncedKyma(ctx context.Context, runtimeClient client.Client,
	key client.ObjectKey,
) (*operatorv1alpha1.Kyma, error) {
	remoteKyma := &operatorv1alpha1.Kyma{}
	if err := runtimeClient.Get(ctx, key, remoteKyma); err != nil {
		return nil, err
	}

	return remoteKyma, nil
}

func DeleteRemotelySyncedKyma(ctx context.Context, controlPlaneClient client.Client,
	kyma *operatorv1alpha1.Kyma,
) error {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy)
	if err != nil {
		return err
	}

	remoteKyma, err := GetRemotelySyncedKyma(ctx, runtimeClient, GetRemoteObjectKey(kyma))
	if err != nil {
		return err
	}

	return runtimeClient.Delete(ctx, remoteKyma)
}

func RemoveFinalizerFromRemoteKyma(ctx context.Context, controlPlaneClient client.Client,
	kyma *operatorv1alpha1.Kyma,
) error {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy)
	if err != nil {
		return err
	}

	remoteKyma, err := GetRemotelySyncedKyma(ctx, runtimeClient, GetRemoteObjectKey(kyma))
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(remoteKyma, operatorv1alpha1.Finalizer)

	return runtimeClient.Update(ctx, remoteKyma)
}

func InitializeKymaSynchronizationContext(ctx context.Context, controlPlaneClient client.Client,
	controlPlaneKyma *operatorv1alpha1.Kyma,
) (*KymaSynchronizationContext, error) {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, client.ObjectKeyFromObject(controlPlaneKyma),
		controlPlaneKyma.Spec.Sync.Strategy)
	if err != nil {
		return nil, err
	}

	sync := &KymaSynchronizationContext{
		controlPlaneClient: controlPlaneClient,
		runtimeClient:      runtimeClient,
		controlPlaneKyma:   controlPlaneKyma,
	}

	return sync, nil
}

func (c *KymaSynchronizationContext) CreateOrUpdateCRD(ctx context.Context, plural string) error {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}
	var err error
	err = c.controlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, operatorv1alpha1.GroupVersion.Group),
	}, crd)

	if err != nil {
		return err
	}

	err = c.runtimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, operatorv1alpha1.GroupVersion.Group),
	}, crdFromRuntime)

	if k8serrors.IsNotFound(err) {
		return c.runtimeClient.Create(ctx, &v1extensions.CustomResourceDefinition{
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

func (c *KymaSynchronizationContext) CreateOrFetchRemoteKyma(ctx context.Context) (*operatorv1alpha1.Kyma, error) {
	kyma := c.controlPlaneKyma
	recorder := adapter.RecorderFromContext(ctx)
	remoteKyma := &operatorv1alpha1.Kyma{}

	remoteKyma.Name = kyma.Name
	remoteKyma.Namespace = c.controlPlaneKyma.Namespace
	if c.controlPlaneKyma.Spec.Sync.Namespace != "" {
		remoteKyma.Namespace = c.controlPlaneKyma.Spec.Sync.Namespace
	}

	err := c.runtimeClient.Get(ctx, client.ObjectKeyFromObject(remoteKyma), remoteKyma)

	if meta.IsNoMatchError(err) {
		recorder.Event(kyma, "Normal", err.Error(), "CRDs are missing in SKR and will be installed")

		if err := c.CreateOrUpdateCRD(ctx, operatorv1alpha1.KymaKind.Plural()); err != nil {
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
			remoteKyma.Spec.Modules = []operatorv1alpha1.Module{}
		}

		err = c.runtimeClient.Create(ctx, remoteKyma)
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
	remoteKyma *operatorv1alpha1.Kyma,
) error {
	recorder := adapter.RecorderFromContext(ctx)

	remoteKyma.Status = c.controlPlaneKyma.Status

	if err := c.runtimeClient.Status().Update(ctx, remoteKyma.SetObservedGeneration()); err != nil {
		recorder.Event(c.controlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma status")
		return err
	}

	if err := c.runtimeClient.Update(ctx, remoteKyma.SetLastSync()); err != nil {
		recorder.Event(c.controlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma last sync annotation")
		return err
	}

	return nil
}

// ReplaceWithVirtualKyma creates a virtual kyma instance from a control plane Kyma and N Remote Kymas,
// merging the module specification in the process.
func (c *KymaSynchronizationContext) ReplaceWithVirtualKyma(kyma *operatorv1alpha1.Kyma,
	remotes ...*operatorv1alpha1.Kyma,
) {
	totalModuleAmount := len(kyma.Spec.Modules)
	for _, remote := range remotes {
		totalModuleAmount += len(remote.Spec.Modules)
	}
	modules := make(map[string]operatorv1alpha1.Module, totalModuleAmount)

	for _, remote := range remotes {
		for _, m := range remote.Spec.Modules {
			modules[m.Name] = m
		}
	}
	for _, m := range kyma.Spec.Modules {
		modules[m.Name] = m
	}

	kyma.Spec.Modules = []operatorv1alpha1.Module{}
	for _, m := range modules {
		kyma.Spec.Modules = append(kyma.Spec.Modules, m)
	}
}

func (c *KymaSynchronizationContext) EnsureNamespaceExists(ctx context.Context, namespace string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	var err error
	if err = c.runtimeClient.Get(ctx, client.ObjectKey{Name: namespace}, ns); k8serrors.IsNotFound(err) {
		return c.runtimeClient.Create(ctx, ns)
	}
	return err
}

func GetRemoteObjectKey(kyma *operatorv1alpha1.Kyma) client.ObjectKey {
	name := kyma.Name
	namespace := kyma.Namespace
	if kyma.Spec.Sync.Namespace != "" {
		namespace = kyma.Spec.Sync.Namespace
	}
	return client.ObjectKey{Namespace: namespace, Name: name}
}

type CatalogSettings struct {
	Name      string
	Namespace string
}

type CatalogEntry struct {
	Defaults *unstructured.Unstructured `json:"defaults"`
	Channel  v1alpha1.Channel           `json:"channel"`
	Target   v1alpha1.Target            `json:"target"`
	Version  string                     `json:"version"`
}

func (c *KymaSynchronizationContext) CreateOrUpdateModuleTemplateCatalog(
	ctx context.Context, catalogSettings CatalogSettings, moduleTemplates *v1alpha1.ModuleTemplateList,
) error {
	catalog := &corev1.ConfigMap{}
	catalog.SetName(catalogSettings.Name)
	catalog.SetNamespace(catalogSettings.Namespace)

	create := false
	err := c.runtimeClient.Get(ctx, client.ObjectKeyFromObject(catalog), catalog)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	create = k8serrors.IsNotFound(err)

	if catalog.Data == nil {
		catalog.Data = make(map[string]string)
	}

	for _, moduleTemplate := range moduleTemplates.Items {
		moduleTemplate := &moduleTemplate
		moduleName := moduleTemplate.GetLabels()[operatorv1alpha1.ModuleName]
		var yml []byte
		var err error

		yml, err = yaml.Marshal(&CatalogEntry{
			Defaults: &moduleTemplate.Spec.Data,
			Channel:  moduleTemplate.Spec.Channel,
			Target:   moduleTemplate.Spec.Target,
			Version:  moduleTemplate.GetLabels()[operatorv1alpha1.ModuleVersion],
		})

		if err != nil {
			return err
		}

		catalog.Data[moduleName] = string(yml)
	}

	if create {
		return c.runtimeClient.Create(ctx, catalog)
	}
	return c.runtimeClient.Update(ctx, catalog)
}

func (c *KymaSynchronizationContext) DeleteModuleTemplateCatalog(
	ctx context.Context, catalogSettings CatalogSettings,
) error {
	catalog := &corev1.Secret{}
	catalog.SetName(catalogSettings.Name)
	catalog.SetNamespace(catalogSettings.Namespace)
	return c.controlPlaneClient.Delete(ctx, catalog)
}
