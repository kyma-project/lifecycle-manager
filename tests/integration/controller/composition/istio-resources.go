package composition

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
	"go.uber.org/zap/zapcore"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	defaultBufferSize = 2048
	logLevel          = 9
)

func CreateIstioResources(
	ctx context.Context,
	restCfg *rest.Config,
	kcpClient client.Client,
) {
	// This k8sClient is used to install external resources
	k8sClient, err := client.New(restCfg, client.Options{Scheme: k8sclientscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	Expect(createNamespace(ctx, shared.IstioNamespace, kcpClient)).To(Succeed())
	Expect(createNamespace(ctx, ControlPlaneNamespace, kcpClient)).To(Succeed())

	istioResources, err := deserializeIstioResources()
	Expect(err).NotTo(HaveOccurred())
	for _, istioResource := range istioResources {
		Expect(k8sClient.Create(ctx, istioResource)).To(Succeed())
	}
	Expect(err).ToNot(HaveOccurred())
}

func GetIstioResources() []*unstructured.Unstructured {
	istioResources, err := deserializeIstioResources()
	Expect(err).NotTo(HaveOccurred())
	return istioResources
}

func createNamespace(ctx context.Context, namespace string, k8sClient client.Client) error {
	ns := &apicorev1.Namespace{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: namespace,
		},
	}
	return k8sClient.Create(ctx, ns)
}

func deserializeIstioResources() ([]*unstructured.Unstructured, error) {
	logr := log.ConfigLogger(logLevel, zapcore.AddSync(GinkgoWriter))
	logf.SetLogger(logr)

	istioResourcesFilePath := filepath.Join(
		integration.GetProjectRoot(),
		"config",
		"samples",
		"tests",
		"istio-test-resources.yaml",
	)

	var istioResourcesList []*unstructured.Unstructured

	file, err := os.Open(istioResourcesFilePath)
	if err != nil {
		return nil, err
	}
	defer func(file io.ReadCloser) {
		err := file.Close()
		if err != nil {
			logr.Error(err, "failed to close test resources", "path", istioResourcesFilePath)
		}
	}(file)
	decoder := machineryaml.NewYAMLOrJSONDecoder(file, defaultBufferSize)
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
