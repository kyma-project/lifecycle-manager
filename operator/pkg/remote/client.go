package remote

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/go-logr/logr"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterClient struct {
	logr.Logger
	DefaultClient client.Client
}

func (cc *ClusterClient) GetRestConfigFromSecret(ctx context.Context, name, namespace string) (*rest.Config, error) {
	kubeConfigSecretList := &v1.SecretList{}
	if err := cc.DefaultClient.List(ctx, kubeConfigSecretList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{labels.KymaName: name}), Namespace: namespace,
	}); err != nil {
		return nil, err
	} else if len(kubeConfigSecretList.Items) < 1 {
		gr := v1.SchemeGroupVersion.WithResource(fmt.Sprintf("secret with label %s", labels.KymaName)).GroupResource()
		return nil, errors.NewNotFound(gr, name)
	}

	kubeConfigSecret := kubeConfigSecretList.Items[0]

	kubeconfigString := string(kubeConfigSecret.Data["config"])

	restConfig, err := cc.GetConfig(kubeconfigString, "")
	if err != nil {
		return nil, err
	}

	return restConfig, err
}

func (cc *ClusterClient) GetConfig(kubeConfig string, explicitPath string) (*rest.Config, error) {
	if kubeConfig != "" {
		// parameter string
		return clientcmd.BuildConfigFromKubeconfigGetter("", func() (config *clientcmdapi.Config, e error) {
			cc.Info("Found config from passed kubeconfig")
			return clientcmd.Load([]byte(kubeConfig))
		})
	}
	// in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		cc.Info("Found config in-cluster")
		return config, err
	}

	// kubeconfig flag
	if flag.Lookup("kubeconfig") != nil {
		if kubeconfig := flag.Lookup("kubeconfig").Value.String(); kubeconfig != "" {
			cc.Info("Found config from flags")
			return clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}

	// env variable
	if len(os.Getenv("KUBECONFIG")) > 0 {
		cc.Info("Found config from env")
		return clientcmd.BuildConfigFromFlags("masterURL", os.Getenv("KUBECONFIG"))
	}

	// default directory + working directory + explicit path -> merged
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = explicitPath

	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error reading current working directory %w", err)
	}

	loadingRules.Precedence = append(loadingRules.Precedence, path.Join(pwd, ".kubeconfig"))
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	cc.Info(fmt.Sprintf("Found config file in: %s", clientConfig.ConfigAccess().GetDefaultFilename()))

	return config, nil
}
