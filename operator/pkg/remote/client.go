package remote

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const KubeConfigKey = "config"

type ClusterClient struct {
	logr.Logger
	DefaultClient client.Client
}

func (cc *ClusterClient) GetRestConfigFromSecret(ctx context.Context, name, namespace string) (*rest.Config, error) {
	kubeConfigSecretList := &v1.SecretList{}
	if err := cc.DefaultClient.List(ctx, kubeConfigSecretList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{v1alpha1.KymaName: name}), Namespace: namespace,
	}); err != nil {
		return nil, err
	} else if len(kubeConfigSecretList.Items) < 1 {
		gr := v1.SchemeGroupVersion.WithResource(fmt.Sprintf("secret with label %s", v1alpha1.KymaName)).GroupResource()

		return nil, errors.NewNotFound(gr, name)
	}

	kubeConfigSecret := kubeConfigSecretList.Items[0]

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigSecret.Data[KubeConfigKey])
	if err != nil {
		return nil, err
	}

	return restConfig, err
}
