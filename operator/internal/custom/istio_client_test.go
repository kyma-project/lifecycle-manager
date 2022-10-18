package custom_test

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

//nolint:gochecknoglobals
var centralComponents = []string{"lifecycle-manager", "module-manager", "compass"}

const (
	defaultBufferSize      = 2048
	istioResourcesFilePath = "../assets/istio-test-resources.yaml"
)

var _ = Describe("configure istio virtual service", Ordered, func() {
	var err error
	var watchers []v1alpha1.Watcher
	var customIstioClient *custom.IstioClient
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		customIstioClient, err = custom.NewVersionedIstioClient(testEnv.Config)
		Expect(err).ToNot(HaveOccurred())
		for idx, component := range centralComponents {
			watchers = append(watchers, createWatcherCR(component, isEven(idx)))
		}
		istioResources, err = deserializeIstioResources()
		Expect(err).NotTo(HaveOccurred())
		for _, istioResource := range istioResources {
			Expect(k8sClient.Create(ctx, istioResource)).To(Succeed())
		}
	})

	AfterAll(func() {
		// clean up istio resources
		for _, istioResource := range istioResources {
			Expect(k8sClient.Delete(ctx, istioResource)).To(Succeed())
		}
	})

	It("configures the KCP virtual service with correct routes provided by the watcher list", func() {
		Expect(customIstioClient.ConfigureVirtualService(ctx, watchers)).To(Succeed())
		for _, watcher := range watchers {
			routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, &watcher)
			Expect(err).ToNot(HaveOccurred())
			Expect(routeReady).To(BeTrue())
		}

	})
})

func deserializeIstioResources() ([]*unstructured.Unstructured, error) {
	var istioResourcesList []*unstructured.Unstructured

	file, err := os.Open(istioResourcesFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := yaml.NewYAMLOrJSONDecoder(file, defaultBufferSize)
	for {
		istioResource := &unstructured.Unstructured{}
		err = decoder.Decode(istioResource)
		if err == nil {
			istioResourcesList = append(istioResourcesList, istioResource)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return istioResourcesList, nil
}

func createWatcherCR(moduleName string, statusOnly bool) v1alpha1.Watcher {
	field := v1alpha1.SpecField
	if statusOnly {
		field = v1alpha1.StatusField
	}
	return v1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1alpha1.WatcherKind),
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sample", moduleName),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				v1alpha1.ManagedBylabel: moduleName,
			},
		},
		Spec: v1alpha1.WatcherSpec{
			ServiceInfo: v1alpha1.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", moduleName),
				Namespace: metav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", moduleName): "true",
			},
			Field: field,
		},
	}
}

func isEven(idx int) bool {
	return idx%2 == 0
}
