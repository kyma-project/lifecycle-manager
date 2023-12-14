package remote

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const KubeConfigKey = "config"

type ClusterClient struct {
	logr.Logger
	DefaultClient client.Client
}

var ErrAccessSecretNotFound = errors.New("access secret not found")

func (cc *ClusterClient) GetRestConfigFromSecret(ctx context.Context, name, namespace string) (*rest.Config, error) {
	kubeConfigSecretList := &apicorev1.SecretList{}
	if err := cc.DefaultClient.List(ctx, kubeConfigSecretList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: name}), Namespace: namespace,
	}); err != nil {
		return nil, fmt.Errorf("failed to list kubeconfig secrets: %w", err)
	} else if len(kubeConfigSecretList.Items) < 1 {
		return nil, fmt.Errorf("secret with label %s=%s %w", shared.KymaName, name, ErrAccessSecretNotFound)
	}

	kubeConfigSecret := kubeConfigSecretList.Items[0]

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigSecret.Data[KubeConfigKey])
	if err != nil {
		return nil, fmt.Errorf("failed to create rest config from kubeconfig: %w", err)
	}

	return restConfig, nil
}
