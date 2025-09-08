package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"go.uber.org/zap/zapcore"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/api"
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
	kcpClient     client.Client
	kcpRESTConfig *rest.Config
	kcpConfig     *[]byte

	skrClient     client.Client
	skrRESTConfig *rest.Config
	skrConfig     *[]byte

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

	kcpConfig, skrConfig, err = getKubeConfigs()
	Expect(err).ToNot(HaveOccurred())
	kcpRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(*kcpConfig)
	kcpRESTConfig.QPS = clientQPS
	kcpRESTConfig.Burst = clientBurst
	Expect(err).ToNot(HaveOccurred())
	skrRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(*skrConfig)
	skrRESTConfig.QPS = clientQPS
	skrRESTConfig.Burst = clientBurst
	Expect(err).ToNot(HaveOccurred())

	kcpClient, err = client.New(kcpRESTConfig, client.Options{Scheme: k8sclientscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	skrClient, err = client.New(skrRESTConfig, client.Options{Scheme: k8sclientscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	Expect(api.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(certmanagerv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(gcertv1alpha1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	SetDefaultEventuallyPollingInterval(interval)
	SetDefaultEventuallyTimeout(EventuallyTimeout)
	SetDefaultConsistentlyDuration(ConsistentDuration)
	SetDefaultConsistentlyPollingInterval(interval)
	// +kubebuilder:scaffold:scheme

	go func() {
		defer GinkgoRecover()
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	go logInstancesWithTicker(logKyma)
	go logInstancesWithTicker(logManifest)
})

func logInstancesWithTicker(logFunc func(context.Context, client.Client)) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logFunc(ctx, kcpClient)
		case <-ctx.Done():
			return
		}
	}
}

func logKyma(ctx context.Context, kcpClient client.Client) {
	kcpKymaList := v1beta2.KymaList{}
	err := kcpClient.List(ctx, &kcpKymaList)
	if err == nil {
		for _, kyma := range kcpKymaList.Items {
			GinkgoWriter.Printf("kyma (%s) in cluster: Spec: %+v, Status: %+v\n", kyma.Name, kyma.Spec,
				kyma.Status)
		}
	} else {
		GinkgoWriter.Printf("error listing kcpKymaList: %v\n", err)
	}
}

func logManifest(ctx context.Context, kcpClient client.Client) {
	manifestList := v1beta2.ManifestList{}
	err := kcpClient.List(ctx, &manifestList)
	if err == nil {
		for _, manifest := range manifestList.Items {
			GinkgoWriter.Printf("manifest (%s) in cluster: Spec: %+v, Status: %+v\n", manifest.Name,
				manifest.Spec, manifest.Status)
		}
	} else {
		GinkgoWriter.Printf("error listing manifestList: %v\n", err)
	}
}

var _ = AfterSuite(func() {
	By("Print out all remaining resources for debugging")
	kcpKymaList := v1beta2.KymaList{}
	err := kcpClient.List(ctx, &kcpKymaList)
	if err == nil {
		for _, kyma := range kcpKymaList.Items {
			GinkgoWriter.Printf("kyma: %v\n", kyma)
		}
	}
	manifestList := v1beta2.ManifestList{}
	err = kcpClient.List(ctx, &manifestList)
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
