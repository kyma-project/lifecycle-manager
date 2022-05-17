package util

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

type ComponentsAssociatedWithTemplate struct {
	ComponentName string
	TemplateHash  *string
}

func GetConfig() (*rest.Config, error) {
	// in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, err
	}

	// kubeconfig flag
	if flag.Lookup("kubeconfig") != nil {
		if kubeconfig := flag.Lookup("kubeconfig").Value.String(); kubeconfig != "" {
			return clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}

	// env variable
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags("masterURL", os.Getenv("KUBECONFIG"))
	}

	// If no in-cluster config, try the default location in the user's home directory
	if home := homedir.HomeDir(); home != "" {
		return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
	}

	return nil, err
}

func SetComponentCRLabels(unstructuredCompCR *unstructured.Unstructured, componentName string, rel operatorv1alpha1.Channel) {
	labelMap := unstructuredCompCR.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	labelMap[labels.ControllerName] = componentName
	labelMap[labels.Channel] = rel
	unstructuredCompCR.Object["metadata"].(map[string]interface{})["labels"] = labelMap
}

func GetGvkAndSpecFromConfigMap(configMap *v1.ConfigMap, componentName string) (*schema.GroupVersionKind, interface{}, error) {
	componentBytes, ok := configMap.Data[componentName]
	if !ok {
		return nil, nil, fmt.Errorf("%s component not found for resource in ConfigMap", componentName)
	}
	componentYaml, err := getTemplatedComponent(componentBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("error during config map template parsing %w", err)
	}

	return &schema.GroupVersionKind{
		Group:   componentYaml["group"].(string),
		Kind:    componentYaml["kind"].(string),
		Version: componentYaml["version"].(string),
	}, componentYaml["spec"], nil
}

func getTemplatedComponent(componentTemplate string) (map[string]interface{}, error) {
	componentYaml := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(componentTemplate), &componentYaml); err != nil {
		return nil, fmt.Errorf("error during config map unmarshal %w", err)
	}
	return componentYaml, nil
}

func AreTemplatesOutdated(logger *logr.Logger, k *operatorv1alpha1.Kyma, templates release.TemplatesByName) bool {
	for componentName, template := range templates {
		for _, condition := range k.Status.Conditions {
			if condition.Reason == componentName && template != nil {
				templateHash := *AsHash(template.Data)
				if templateHash != condition.TemplateHash {
					logger.Info("detected outdated template",
						"condition", condition.Reason,
						"template", template.Name,
						"templateHash", templateHash,
						"oldHash", condition.TemplateHash,
					)
					return true
				}
			}
		}
	}
	return false
}

func AsHash(o interface{}) *string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", o)))
	v := fmt.Sprintf("%x", h.Sum(nil))
	return &v
}
