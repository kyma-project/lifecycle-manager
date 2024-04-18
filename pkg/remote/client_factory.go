package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	kymactx "github.com/kyma-project/lifecycle-manager/internal/controller/kyma/context"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SkrContextFactory interface {
	Get(ctx context.Context) (*SkrContext, error)
	Init(ctx context.Context, kyma *v1beta2.Kyma) error
}

type KymaSkrContextFactory struct {
	clientCache *ClientCache
	kcpClient   Client
}

func NewKymaSkrContextFactory(kcpClient Client, clientCache *ClientCache) *KymaSkrContextFactory {
	return &KymaSkrContextFactory{
		clientCache: clientCache,
		kcpClient:   kcpClient,
	}
}

const KubeConfigKey = "config"

var ErrAccessSecretNotFound = errors.New("access secret not found")

func (k *KymaSkrContextFactory) Init(ctx context.Context, kyma *v1beta2.Kyma) error {
	kubeConfigSecretList := &apicorev1.SecretList{}
	if err := k.kcpClient.List(ctx, kubeConfigSecretList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kyma.Name}), Namespace: kyma.Namespace,
	}); err != nil {
		return fmt.Errorf("failed to list kubeconfig secrets: %w", err)
	} else if len(kubeConfigSecretList.Items) < 1 {
		return fmt.Errorf("secret with label %s=%s %w", shared.KymaName, kyma.Name, ErrAccessSecretNotFound)
	}

	kubeConfigSecret := kubeConfigSecretList.Items[0]

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigSecret.Data[KubeConfigKey])
	if err != nil {
		return fmt.Errorf("failed to create rest config from kubeconfig: %w", err)
	}

	restConfig.QPS = k.kcpClient.Config().QPS
	restConfig.Burst = k.kcpClient.Config().Burst

	clnt, err := client.New(restConfig, client.Options{Scheme: k.kcpClient.Scheme()})
	if err != nil {
		return fmt.Errorf("failed to create lookup client: %w", err)
	}

	skr := NewClientWithConfig(clnt, restConfig)

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
	if err := skr.Patch(ctx, namespace, patch, patchOpts); err != nil {
		return fmt.Errorf("failed to ensure remote namespace exists: %w", err)
	}

	key := types.NamespacedName{Name: kyma.Name, Namespace: kyma.Namespace}
	k.clientCache.Add(key, skr)
	return nil
}

func (k *KymaSkrContextFactory) Get(ctx context.Context) (*SkrContext, error) {
	kymaName, err := kymactx.Get(ctx)
	if err != nil {
		// TODO wrap error
		return nil, err
	}

	skrClient := k.clientCache.Get(kymaName)

	kymaClient := &SkrContext{
		Client: skrClient,
	}

	return kymaClient, nil
}
