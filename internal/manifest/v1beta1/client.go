package v1beta1

import (
	"context"
	"errors"
	"fmt"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"strconv"
	"strings"

	manifestv1beta1 "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/pkg/types"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var ErrKubeconfigFetchFailed = errors.New("could not fetch kubeconfig")

type ClusterClient struct {
	DefaultClient client.Client
}

var ErrMoreThanOneSecretFound = errors.New("more than one secret found")

func (cc *ClusterClient) GetRESTConfig(
	ctx context.Context, kymaOwner, kymaNameLabel, namespace string,
) (*rest.Config, error) {
	kubeConfigSecretList := &v1.SecretList{}
	groupResource := v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets)).GroupResource()
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{kymaNameLabel: kymaOwner})
	err := cc.DefaultClient.List(
		ctx, kubeConfigSecretList, &client.ListOptions{LabelSelector: labelSelector, Namespace: namespace},
	)
	if err != nil {
		return nil, err
	}
	kubeConfigSecret := &v1.Secret{}
	if len(kubeConfigSecretList.Items) < 1 {
		key := client.ObjectKey{Name: kymaOwner, Namespace: namespace}
		if err := cc.DefaultClient.Get(ctx, key, kubeConfigSecret); err != nil {
			return nil, fmt.Errorf("could not get by key (%s) or selector (%s): %w",
				key, labelSelector.String(), ErrKubeconfigFetchFailed)
		}
	} else {
		kubeConfigSecret = &kubeConfigSecretList.Items[0]
	}
	if len(kubeConfigSecretList.Items) > 1 {
		return nil, k8serrors.NewConflict(groupResource, kymaOwner, fmt.Errorf(
			"could not safely identify the rest config source: %w", ErrMoreThanOneSecretFound))
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigSecret.Data["config"])
	if err != nil {
		return nil, err
	}
	return restConfig, err
}

func WithClientCacheKey() declarative.WithClientCacheKeyOption {
	cacheKey := func(ctx context.Context, resource declarative.Object) (any, bool) {
		logger := log.FromContext(ctx)

		manifest := resource.(*manifestv1beta1.Manifest)
		labelValue, err := internal.GetResourceLabel(resource, manifestv1beta1.KymaName)
		objectKey := client.ObjectKeyFromObject(resource)
		var labelErr *types.LabelNotFoundError
		if errors.As(err, &labelErr) {
			logger.V(internal.DebugLogLevel).Info(
				"client can not been cached due to lack of expected label",
				"resource", objectKey)
			return nil, false
		}
		cacheKey := GenerateCacheKey(labelValue, strconv.FormatBool(manifest.Spec.Remote), manifest.GetNamespace())
		return cacheKey, true
	}
	return declarative.WithClientCacheKeyOption{ClientCacheKeyFn: cacheKey}
}

func GenerateCacheKey(values ...string) string {
	return strings.Join(values, "|")
}
