package watcher_test

import (
	"context"
	"path/filepath"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	pkgApi "github.com/kyma-project/lifecycle-manager/pkg/api"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	controlPlaneClient client.Client        //nolint:gochecknoglobals
	controlPlaneEnv    *envtest.Environment //nolint:gochecknoglobals
	ctx                context.Context      //nolint:gochecknoglobals
	cancel             context.CancelFunc   //nolint:gochecknoglobals
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "CertificateManager Sync")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)

	By("bootstrapping control plane test environment")
	externalCRDs, err := AppendExternalCRDs(
		filepath.Join(integration.GetProjectRoot(), "config", "samples", "tests", "crds"),
		"cert-manager-v1.10.1.crds.yaml",
		"istio-v1.17.1.crds.yaml")
	Expect(err).ToNot(HaveOccurred())
	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")},
		CRDs:                  externalCRDs,
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(pkgApi.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(certmanagerv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	controlPlaneClient, err = client.New(cfg, client.Options{Scheme: k8sclientscheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(controlPlaneClient).NotTo(BeNil())

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
	Expect(err).NotTo(HaveOccurred())
})
