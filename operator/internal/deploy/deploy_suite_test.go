package deploy_test

import (
	"context"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	"k8s.io/client-go/rest"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	lifecyclemgrapi "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
	//+kubebuilder:scaffold:imports
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Deploy Suite")
}

var (
	ctx               context.Context         //nolint:gochecknoglobals
	kcpTestEnv        *envtest.Environment    //nolint:gochecknoglobals
	skrTestEnv        *envtest.Environment    //nolint:gochecknoglobals
	skrCfg            *rest.Config            //nolint:gochecknoglobals
	kcpCfg            *rest.Config            //nolint:gochecknoglobals
	kcpClient         client.Client           //nolint:gochecknoglobals
	skrClient         client.Client           //nolint:gochecknoglobals
	webhookMgr        *deploy.SKRChartManager //nolint:gochecknoglobals
	remoteClientCache *remote.ClientCache     //nolint:gochecknoglobals
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
	err = k8syaml.NewYAMLOrJSONDecoder(resp.Body, 2048).Decode(kymaCrd)
	Expect(err).ToNot(HaveOccurred())
	resp, err = http.Get("https://raw.githubusercontent.com/kyma-project/lifecycle-manager/" +
		"main/operator/config/crd/bases/operator.kyma-project.io_watchers.yaml")
	Expect(err).ToNot(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	defer resp.Body.Close()
	watcherCrd := &apiextv1.CustomResourceDefinition{}
	err = k8syaml.NewYAMLOrJSONDecoder(resp.Body, 2048).Decode(watcherCrd)
	Expect(err).ToNot(HaveOccurred())

	By("bootstrapping test environment for webhook deployment tests")
	kcpTestEnv = &envtest.Environment{
		CRDs: []*apiextv1.CustomResourceDefinition{kymaCrd, watcherCrd},
	}

	kcpCfg, err := kcpTestEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(kcpCfg).NotTo(BeNil())

	Expect(lifecyclemgrapi.AddToScheme(scheme.Scheme)).To(Succeed())

	//+kubebuilder:scaffold:scheme

	kcpClient, err = client.New(kcpCfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(kcpClient).NotTo(BeNil())

	Expect(deploy.CreateLoadBalancer(ctx, kcpClient)).To(Succeed())

	webhookMgr = deploy.NewSKRChartManager(webhookChartPath, memoryLimits, cpuLimits, true)
	Expect(err).NotTo(HaveOccurred())
	Expect(webhookMgr).NotTo(BeNil())

	remoteClientCache = remote.NewClientCache()
	skrEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
	}
	skrCfg, err := skrEnv.Start()
	Expect(skrCfg).NotTo(BeNil())
	Expect(err).NotTo(HaveOccurred())

	var authUser *envtest.AuthenticatedUser
	authUser, err = skrEnv.AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, skrCfg)
	Expect(err).NotTo(HaveOccurred())

	remote.LocalClient = func() *rest.Config {
		return authUser.Config()
	}

	skrClient, err = client.New(authUser.Config(), client.Options{Scheme: kcpClient.Scheme()})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(kcpTestEnv.Stop()).To(Succeed())
})
