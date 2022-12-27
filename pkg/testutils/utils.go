package testutils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/onsi/gomega"

	v12 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	randomStringLength = 8
	letterBytes        = "abcdefghijklmnopqrstuvwxyz"
	defaultBufferSize  = 2048
	httpClientTimeout  = 2 * time.Second
	Timeout            = time.Second * 10
	Interval           = time.Millisecond * 250
)

func NewTestKyma(name string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: v1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       string(v1alpha1.KymaKind),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name + RandString(randomStringLength),
			Namespace: v1.NamespaceDefault,
		},
		Spec: v1alpha1.KymaSpec{
			Modules: []v1alpha1.Module{},
			Channel: v1alpha1.DefaultChannel,
		},
	}
}

func NewUniqModuleName() string {
	return RandString(randomStringLength)
}

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec
	}
	return string(b)
}

func DeployModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1alpha1.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(kcpClient.Create(ctx, template)).To(gomega.Succeed())
	}
}

func DeleteModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1alpha1.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Expect(kcpClient.Delete(ctx, template)).To(gomega.Succeed())
	}
}

func GetKyma(ctx context.Context, testClient client.Client, name, namespace string) (*v1alpha1.Kyma, error) {
	kymaInCluster := &v1alpha1.Kyma{}
	if namespace == "" {
		namespace = v1.NamespaceDefault
	}
	err := testClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, kymaInCluster)
	if err != nil {
		return nil, err
	}
	return kymaInCluster, nil
}

func IsKymaInState(ctx context.Context, kcpClient client.Client, kymaName string, state v1alpha1.State) func() bool {
	return func() bool {
		kymaFromCluster, err := GetKyma(ctx, kcpClient, kymaName, "")
		if err != nil || kymaFromCluster.Status.State != state {
			return false
		}
		return true
	}
}

func ParseRemoteCRDs(testCrdURLs []string) ([]*v12.CustomResourceDefinition, error) {
	var crds []*v12.CustomResourceDefinition
	var httpResponse *http.Response
	for _, testCrdURL := range testCrdURLs {
		_, err := url.Parse(testCrdURL)
		if err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testCrdURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed pulling content for URL (%s) :%w", testCrdURL, err)
		}
		httpClient := &http.Client{Timeout: httpClientTimeout}
		httpResponse, err = httpClient.Do(request)
		if err != nil {
			return nil, err
		}
		if httpResponse.StatusCode != http.StatusOK {
			//nolint:goerr113
			return nil, fmt.Errorf("failed pulling content for URL (%s) with status code: %d",
				testCrdURL, httpResponse.StatusCode)
		}

		decoder := yaml.NewYAMLOrJSONDecoder(httpResponse.Body, defaultBufferSize)
		for {
			crd := &v12.CustomResourceDefinition{}
			err = decoder.Decode(crd)
			if err == nil {
				crds = append(crds, crd)
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
	}
	defer func() {
		_ = httpResponse.Body.Close()
	}()
	return crds, nil
}

func ModuleTemplateFactory(module v1alpha1.Module, data unstructured.Unstructured) (*v1alpha1.ModuleTemplate, error) {
	var moduleTemplate v1alpha1.ModuleTemplate
	err := readModuleTemplate(module, &moduleTemplate)
	if err != nil {
		return &moduleTemplate, err
	}
	moduleTemplate.Name = module.Name
	moduleTemplate.Labels[v1alpha1.ModuleName] = module.Name
	moduleTemplate.Labels[v1alpha1.ControllerName] = module.ControllerName
	moduleTemplate.Spec.Channel = module.Channel
	if data.GetKind() != "" {
		moduleTemplate.Spec.Data = data
	}
	return &moduleTemplate, nil
}

func readModuleTemplate(module v1alpha1.Module, moduleTemplate *v1alpha1.ModuleTemplate) error {
	var template string
	switch module.ControllerName {
	case "manifest":
		template = "operator_v1alpha1_moduletemplate_skr-module.yaml"
	default:
		template = "operator_v1alpha1_moduletemplate_kcp-module.yaml"
	}
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(filepath.Dir(filename), "../../config/samples/component-integration-installed", template)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(moduleFile, &moduleTemplate)
	return err
}
