//nolint:gochecknoglobals
package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zapcore"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	kcpConfigEnvVar, skrConfigEnvVar = "KCP_KUBECONFIG", "SKR_KUBECONFIG"
	clientQPS, clientBurst           = 1000, 2000
)

var errEmptyEnvVar = errors.New("environment variable is empty")

var (
	controlPlaneClient     client.Client
	controlPlaneRESTConfig *rest.Config
	controlPlaneConfig     *[]byte

	runtimeClient     client.Client
	runtimeRESTConfig *rest.Config
	runtimeConfig     *[]byte

	ctx    context.Context
	cancel context.CancelFunc
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))

	By("bootstrapping test environment")

	kcpModuleCRD := &apiextensionsv1.CustomResourceDefinition{}
	modulePath := filepath.Join("../..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(machineryaml.Unmarshal(moduleFile, &kcpModuleCRD)).To(Succeed())

	controlPlaneConfig, runtimeConfig, err = getKubeConfigs()
	Expect(err).ToNot(HaveOccurred())
	controlPlaneRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(*controlPlaneConfig)
	controlPlaneRESTConfig.QPS = clientQPS
	controlPlaneRESTConfig.Burst = clientBurst
	Expect(err).ToNot(HaveOccurred())
	runtimeRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(*runtimeConfig)
	runtimeRESTConfig.QPS = clientQPS
	runtimeRESTConfig.Burst = clientBurst
	Expect(err).ToNot(HaveOccurred())

	controlPlaneClient, err = client.New(controlPlaneRESTConfig, client.Options{Scheme: k8sclientscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	runtimeClient, err = client.New(runtimeRESTConfig, client.Options{Scheme: k8sclientscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	Expect(api.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	SetDefaultEventuallyPollingInterval(interval)
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultConsistentlyDuration(timeout)
	SetDefaultConsistentlyPollingInterval(interval)
	//+kubebuilder:scaffold:scheme

	go func() {
		defer GinkgoRecover()
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("Print out all remaining resources for debugging")
	kcpKymaList := v1beta2.KymaList{}
	err := controlPlaneClient.List(ctx, &kcpKymaList)
	if err == nil {
		for _, kyma := range kcpKymaList.Items {
			GinkgoWriter.Printf("kyma: %v\n", kyma)
		}
	}
	manifestList := v1beta2.ManifestList{}
	err = controlPlaneClient.List(ctx, &manifestList)
	if err == nil {
		for _, manifest := range manifestList.Items {
			GinkgoWriter.Printf("manifest: %v\n", manifest)
		}
	}
	cancel()
})

func getKubeConfigs() (*[]byte, *[]byte, error) {
	controlPlaneConfigFile := os.Getenv(kcpConfigEnvVar)
	if controlPlaneConfigFile == "" {
		return nil, nil, fmt.Errorf("%w: %s", errEmptyEnvVar, kcpConfigEnvVar)
	}
	controlPlaneConfig, err := os.ReadFile(controlPlaneConfigFile)
	if err != nil {
		return nil, nil, err
	}

	runtimeConfigFile := os.Getenv(skrConfigEnvVar)
	if runtimeConfigFile == "" {
		return nil, nil, fmt.Errorf("%w: %s", errEmptyEnvVar, skrConfigEnvVar)
	}
	runtimeConfig, err := os.ReadFile(runtimeConfigFile)
	if err != nil {
		return nil, nil, err
	}

	return &controlPlaneConfig, &runtimeConfig, nil
}
