package manifest

import (
	"context"
	"errors"
	"fmt"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterClient struct {
	DefaultClient client.Client
}

var ErrMoreThanOneSecretFound = errors.New("more than one secret found")

func (cc *ClusterClient) GetRESTConfig(
	ctx context.Context, kymaOwner, kymaNameLabel, namespace string,
) (*rest.Config, error) {
	kubeConfigSecretList := &apicorev1.SecretList{}
	groupResource := apicorev1.SchemeGroupVersion.WithResource(string(apicorev1.ResourceSecrets)).GroupResource()
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{kymaNameLabel: kymaOwner})
	err := cc.DefaultClient.List(
		ctx, kubeConfigSecretList, &client.ListOptions{LabelSelector: labelSelector, Namespace: namespace},
	)
	if err != nil {
		return nil,
			fmt.Errorf("failed to list resources by {LabelSelector: %v, Namespace: %v}: %w", labelSelector, namespace,
				err)
	}
	kubeConfigSecret := &apicorev1.Secret{}
	if len(kubeConfigSecretList.Items) < 1 {
		key := client.ObjectKey{Name: kymaOwner, Namespace: namespace}
		if err := cc.DefaultClient.Get(ctx, key, kubeConfigSecret); err != nil {
			return nil, fmt.Errorf("could not get by key (%s) or selector (%s): %w",
				key, labelSelector.String(), declarativev2.ErrAccessSecretNotFound)
		}
	} else {
		kubeConfigSecret = &kubeConfigSecretList.Items[0]
	}
	if len(kubeConfigSecretList.Items) > 1 {
		return nil, apierrors.NewConflict(groupResource, kymaOwner, fmt.Errorf(
			"could not safely identify the rest config source: %w", ErrMoreThanOneSecretFound))
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigSecret.Data["config"])
	if err != nil {
		return nil, fmt.Errorf("failed to get rest configuration from kubeconfig: %w", err)
	}
	return restConfig, nil
}
