package deploy_test

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
	//+kubebuilder:scaffold:imports
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Deploy Suite")
}

var (
	ctx        context.Context         //nolint:gochecknoglobals
	testEnv    *envtest.Environment    //nolint:gochecknoglobals
	k8sClient  client.Client           //nolint:gochecknoglobals
	webhookMgr *deploy.SKRChartManager //nolint:gochecknoglobals
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx = context.Background()

	By("preparing required CRDs")
	resp, err := http.Get("https://raw.githubusercontent.com/kyma-project/lifecycle-manager/" +
		"main/operator/config/crd/bases/operator.kyma-project.io_kymas.yaml")
	Expect(err).ToNot(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	defer resp.Body.Close()
	kymaCrd := &apiextv1.CustomResourceDefinition{}
	err = yaml.NewYAMLOrJSONDecoder(resp.Body, 2048).Decode(kymaCrd)
	Expect(err).ToNot(HaveOccurred())

	By("bootstrapping test environment for webhook deployment tests")
	testEnv = &envtest.Environment{
		CRDs: []*apiextv1.CustomResourceDefinition{kymaCrd},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(kyma.AddToScheme(scheme.Scheme)).To(Succeed())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	Expect(deploy.CreateLoadBalancer(ctx, k8sClient)).To(Succeed())

	webhookMgr, err = deploy.NewSKRChartManager(ctx, k8sClient, webhookChartPath, memoryLimits, cpuLimits)
	Expect(err).NotTo(HaveOccurred())
	Expect(webhookMgr).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})
