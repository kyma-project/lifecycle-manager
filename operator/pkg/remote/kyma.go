package remote

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

type KymaSynchronizationContext struct {
	controlPlaneClient client.Client
	runtimeClient      client.Client
	controlPlaneKyma   *operatorv1alpha1.Kyma
}

func NewRemoteClient(ctx context.Context, controlPlaneClient client.Client, name, namespace string) (client.Client, error) {
	cc := ClusterClient{
		DefaultClient: controlPlaneClient,
		Logger:        log.FromContext(ctx),
	}

	rc, err := cc.GetRestConfigFromSecret(ctx, name, namespace)
	if err != nil {
		return nil, err
	}

	remoteClient, err := client.New(rc, client.Options{Scheme: controlPlaneClient.Scheme()})
	if err != nil {
		return nil, err
	}
	return remoteClient, nil
}

func GetRemotelySyncedKyma(ctx context.Context, runtimeClient client.Client, key client.ObjectKey) (*operatorv1alpha1.Kyma, error) {
	remoteKyma := &operatorv1alpha1.Kyma{}
	if err := runtimeClient.Get(ctx, key, remoteKyma); err != nil {
		return nil, err
	}
	return remoteKyma, nil
}

func DeleteRemotelySyncedKyma(ctx context.Context, controlPlaneClient client.Client, key client.ObjectKey) error {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, key.Name, key.Namespace)
	if err != nil {
		return err
	}
	remoteKyma, err := GetRemotelySyncedKyma(ctx, runtimeClient, key)
	if err != nil {
		return err
	}
	return runtimeClient.Delete(ctx, remoteKyma)
}

func RemoveFinalizerFromRemoteKyma(ctx context.Context, controlPlaneClient client.Client, key client.ObjectKey) error {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, key.Name, key.Namespace)
	if err != nil {
		return err
	}
	remoteKyma, err := GetRemotelySyncedKyma(ctx, runtimeClient, key)
	if err != nil {
		return err
	}
	controllerutil.RemoveFinalizer(remoteKyma, labels.Finalizer)
	return runtimeClient.Update(ctx, remoteKyma)
}

func InitializeKymaSynchronizationContext(ctx context.Context, controlPlaneClient client.Client, controlPlaneKyma *operatorv1alpha1.Kyma) (*KymaSynchronizationContext, error) {
	runtimeClient, err := NewRemoteClient(ctx, controlPlaneClient, controlPlaneKyma.Name, controlPlaneKyma.Namespace)
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
	err := c.runtimeClient.Get(ctx, client.ObjectKeyFromObject(kyma), remoteKyma)

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
		remoteKyma.Name = kyma.Name
		remoteKyma.Namespace = kyma.Namespace
		remoteKyma.Spec = *kyma.Spec.DeepCopy()
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

func (c *KymaSynchronizationContext) SynchronizeRemoteKyma(ctx context.Context, remoteKyma *operatorv1alpha1.Kyma) (bool, error) {
	kyma := c.controlPlaneKyma
	recorder := adapter.RecorderFromContext(ctx)
	// check finalizer
	if !controllerutil.ContainsFinalizer(remoteKyma, labels.Finalizer) {
		controllerutil.AddFinalizer(remoteKyma, labels.Finalizer)
	}

	previousRemoteObservedGeneration := remoteKyma.Status.ObservedGeneration
	if remoteKyma.Status.ObservedGeneration != remoteKyma.GetGeneration() {
		remoteKyma.Status.ObservedGeneration = remoteKyma.GetGeneration()
		if err := c.runtimeClient.Status().Update(ctx, remoteKyma); err != nil {
			return false, err
		}

		kyma.Spec = remoteKyma.Spec
		err := c.controlPlaneClient.Update(ctx, kyma)
		if err != nil {
			// revert remote observed generation
			remoteKyma.Status.ObservedGeneration = previousRemoteObservedGeneration
			if err := c.runtimeClient.Status().Update(ctx, remoteKyma); err != nil {
				return false, err
			}
			recorder.Event(remoteKyma, "Warning", err.Error(), "Client could not update "+
				"Control Plane Kyma while syncing with remote Kyma")
		}
		// only re-queue if control plane Kyma is updated
		return true, err
	}

	if remoteKyma.Status.State != kyma.Status.State {
		remoteKyma.Status.State = kyma.Status.State
		err := c.runtimeClient.Status().Update(ctx, remoteKyma)
		if err != nil {
			return false, err
		}
	}

	lastSyncDate := time.Now().Format(time.RFC3339)
	if remoteKyma.Annotations == nil {
		remoteKyma.Annotations = make(map[string]string)
	}
	remoteKyma.Annotations[labels.LastSync] = lastSyncDate

	return false, c.runtimeClient.Update(ctx, remoteKyma)
}
