package remote

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
)

type SkrContextProvider interface {
	Get(kyma types.NamespacedName) (*SkrContext, error)
	Init(ctx context.Context, kyma types.NamespacedName) error
	InvalidateCache(kyma types.NamespacedName)
}

type KymaSkrContextProvider struct {
	clientCache *ClientCache
	kcpClient   Client
	event       event.Event
}

func NewKymaSkrContextProvider(kcpClient Client, clientCache *ClientCache, event event.Event) *KymaSkrContextProvider {
	return &KymaSkrContextProvider{
		clientCache: clientCache,
		kcpClient:   kcpClient,
		event:       event,
	}
}

const kubeConfigKey = "config"

var ErrSkrClientContextNotFound = errors.New("skr client context not found")

func (k *KymaSkrContextProvider) Init(ctx context.Context, kyma types.NamespacedName) error {
	if k.clientCache.Contains(kyma) {
		return nil
	}

	kubeConfigSecretList := &apicorev1.SecretList{}
	err := k.kcpClient.List(ctx, kubeConfigSecretList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kyma.Name}), Namespace: kyma.Namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to list kubeconfig secrets: %w", err)
	} else if len(kubeConfigSecretList.Items) < 1 {
		return fmt.Errorf("secret with label %s=%s %w", shared.KymaName, kyma.Name, common.ErrAccessSecretNotFound)
	}

	kubeConfigSecret := kubeConfigSecretList.Items[0]

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigSecret.Data[kubeConfigKey])
	if err != nil {
		return fmt.Errorf("failed to create rest config from kubeconfig: %w", err)
	}

	restConfig.QPS = k.kcpClient.Config().QPS
	restConfig.Burst = k.kcpClient.Config().Burst

	// Required to prevent memory leak by avoiding caching in transport.tlsTransportCache. skrClients are cached anyways.
	restConfig.Proxy = http.ProxyFromEnvironment

	clnt, err := client.New(restConfig, client.Options{Scheme: k.kcpClient.Scheme()})
	if err != nil {
		return fmt.Errorf("failed to create lookup client: %w", err)
	}

	skrClient := NewClientWithConfig(clnt, restConfig)
	k.clientCache.Add(kyma, skrClient)

	return nil
}

func (k *KymaSkrContextProvider) Get(kyma types.NamespacedName) (*SkrContext, error) {
	skrClient := k.clientCache.Get(kyma)
	if skrClient == nil {
		return nil, ErrSkrClientContextNotFound
	}

	return NewSkrContext(skrClient, k.event), nil
}

func (k *KymaSkrContextProvider) InvalidateCache(kyma types.NamespacedName) {
	k.clientCache.Delete(kyma)
}
