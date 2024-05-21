package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

type SkrContextFactory interface {
	Get(kyma types.NamespacedName) (*SkrContext, error)
	Init(ctx context.Context, kyma types.NamespacedName) error
	InvalidateCache(kyma types.NamespacedName)
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

var (
	ErrAccessSecretNotFound     = errors.New("access secret not found")
	ErrSkrClientContextNotFound = errors.New("skr client context not found")
)

func (k *KymaSkrContextFactory) Init(ctx context.Context, kyma types.NamespacedName) error {
	if k.clientCache.Contains(kyma) {
		return nil
	}

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

	skrClient := NewClientWithConfig(clnt, restConfig)

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
	if err := skrClient.Patch(ctx, namespace, patch, patchOpts); err != nil {
		return fmt.Errorf("failed to ensure remote namespace exists: %w", err)
	}

	k.clientCache.Add(kyma, skrClient)
	return nil
}

func (k *KymaSkrContextFactory) Get(kyma types.NamespacedName) (*SkrContext, error) {
	skrClient := k.clientCache.Get(kyma)
	if skrClient == nil {
		return nil, ErrSkrClientContextNotFound
	}

	return &SkrContext{Client: skrClient}, nil
}

func (k *KymaSkrContextFactory) InvalidateCache(kyma types.NamespacedName) {
	k.clientCache.Delete(kyma)
}
