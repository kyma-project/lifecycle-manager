package remote

import (
	"context"
	"fmt"
	"time"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	corev1 "k8s.io/api/core/v1"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type KymaSynchronizationContext struct {
	controlPlaneClient client.Client
	runtimeClient      client.Client
	controlPlaneKyma   *operatorv1alpha1.Kyma
}

func NewRemoteClient(ctx context.Context, controlPlaneClient client.Client, key client.ObjectKey, //nolint:ireturn
	strategy operatorv1alpha1.SyncStrategy,
) (client.Client, error) {
	clusterClient := ClusterClient{
		DefaultClient: controlPlaneClient,
		Logger:        log.FromContext(ctx),
	}

	var restConfig *rest.Config

	var err error

	switch strategy {
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

func (c *KymaSynchronizationContext) CreateCRD(ctx context.Context) error {
	crd := &v1extensions.CustomResourceDefinition{}
	if err := c.controlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", operatorv1alpha1.KymaPlural, operatorv1alpha1.GroupVersion.Group),
	}, crd); err != nil {
		return err
	}

	return c.runtimeClient.Create(ctx, &v1extensions.CustomResourceDefinition{
		ObjectMeta: v1.ObjectMeta{Name: crd.Name, Namespace: crd.Namespace}, Spec: crd.Spec,
	})
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

		if err := c.CreateCRD(ctx); err != nil {
			return nil, err
		}

		recorder.Event(kyma, "Normal", "CRDInstallation", "CRDs were installed to SKR")
		// the NoMatch error we previously encountered is now fixed through the CRD installation
		err = nil
	}

	if errors.IsNotFound(err) {
		if err := c.EnsureNamespaceExists(ctx, remoteKyma.Namespace); err != nil {
			recorder.Event(kyma, "Warning", "RemoteKymaInstallation",
				fmt.Sprintf("namespace %s could not be synced", remoteKyma.Namespace))

			return nil, err
		}

		kyma.Spec.DeepCopyInto(&remoteKyma.Spec)

		err = c.runtimeClient.Create(ctx, remoteKyma)
		if err != nil {
			recorder.Event(kyma, "Normal", "CRDInstallation", "CRDs were installed to SKR")

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
) (bool, error) {
	recorder := adapter.RecorderFromContext(ctx)
	// check finalizer
	if !controllerutil.ContainsFinalizer(remoteKyma, operatorv1alpha1.Finalizer) {
		controllerutil.AddFinalizer(remoteKyma, operatorv1alpha1.Finalizer)
	}

	if remoteKyma.Status.ObservedGeneration != remoteKyma.GetGeneration() {
		// remote is new, lets update the control plane
		remoteKyma.Status.ObservedGeneration = remoteKyma.GetGeneration()
		if err := c.runtimeClient.Status().Update(ctx, remoteKyma); err != nil {
			recorder.Event(c.controlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma status")

			return true, err
		}

		c.controlPlaneKyma.Spec = remoteKyma.Spec
		if err := c.controlPlaneClient.Update(ctx, c.controlPlaneKyma); err != nil {
			recorder.Event(c.controlPlaneKyma, "Warning", err.Error(), "could not update control clane kyma")

			return true, err
		}

		// as we have updated the control plane, we will requeue the object
		return true, nil
	}

	if remoteKyma.Status.State != c.controlPlaneKyma.Status.State {
		// control plane and runtime spec are in sync, but the status got updated in the control plane
		remoteKyma.Status.State = c.controlPlaneKyma.Status.State

		err := c.runtimeClient.Status().Update(ctx, remoteKyma)
		if err != nil {
			recorder.Event(c.controlPlaneKyma, "Warning", err.Error(), "could not update runtime kyma status")

			return false, err
		}
	}

	// this is an additional update on the runtime and might not be worth it
	lastSyncDate := time.Now().Format(time.RFC3339)

	if remoteKyma.Annotations == nil {
		remoteKyma.Annotations = make(map[string]string)
	}

	remoteKyma.Annotations[operatorv1alpha1.LastSync] = lastSyncDate

	return false, c.runtimeClient.Update(ctx, remoteKyma)
}

func (c *KymaSynchronizationContext) EnsureNamespaceExists(ctx context.Context, namespace string) error {
	ns := &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: namespace}}
	var err error
	if err = c.runtimeClient.Get(ctx, client.ObjectKey{Name: namespace}, ns); errors.IsNotFound(err) {
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
