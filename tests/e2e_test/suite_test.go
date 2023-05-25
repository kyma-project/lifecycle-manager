///go:build e2e
//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/rest"

	//nolint:gci
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	kcpConfigEnvVar = "KCP_KUBECONFIG"
	skrConfigEnvVar = "SKR_KUBECONFIG"
)

var errEmptyEnvVar = errors.New("environment variable is empty;")

var (
	controlPlaneEnv        *envtest.Environment //nolint:gochecknoglobals
	controlPlaneClient     client.Client        //nolint:gochecknoglobals
	controlPlaneRESTConfig *rest.Config         //nolint:gochecknoglobals
	controlPlaneConfig     *[]byte              //nolint:gochecknoglobals

	runtimeClient     client.Client //nolint:gochecknoglobals
	runtimeRESTConfig *rest.Config  //nolint:gochecknoglobals
	runtimeConfig     *[]byte       //nolint:gochecknoglobals

	ctx    context.Context    //nolint:gochecknoglobals
	cancel context.CancelFunc //nolint:gochecknoglobals

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

	externalCRDs := AppendExternalCRDs(
		filepath.Join("../..", "config", "samples", "tests", "crds"),
		"cert-manager-v1.10.1.crds.yaml",
		"istio-v1.17.1.crds.yaml")

	// kcpModule CRD
	controlPlaneCrd := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("../..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml2.Unmarshal(moduleFile, &controlPlaneCrd)).To(Succeed())

	// k8s configs
	controlPlaneConfig, runtimeConfig, err = getKubeConfigs()
	Expect(err).ToNot(HaveOccurred())
	existingCluster := true
	controlPlaneRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(*controlPlaneConfig)
	Expect(err).ToNot(HaveOccurred())
	runtimeRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(*runtimeConfig)
	Expect(err).ToNot(HaveOccurred())

	Expect(err).NotTo(HaveOccurred())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("../..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{controlPlaneCrd}, externalCRDs...),
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &existingCluster,
		Config:                controlPlaneRESTConfig,
	}
	controlPlaneClient, err = client.New(controlPlaneRESTConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	runtimeClient, err = client.New(runtimeRESTConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	_, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(controlPlaneEnv.Config).NotTo(BeNil())

	Expect(api.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	go func() {
		defer GinkgoRecover()
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()

	err := controlPlaneEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
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
