package watcher_test

import (
	"context"
	"path/filepath"
	"testing"

	operatorv1alpha1 "github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	certManagerV1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	moduleManagerV1alpha1 "github.com/kyma-project/module-manager/operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes/scheme"
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
	RunSpecs(t, "Certificate Sync")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)

	By("bootstrapping control plane test environment")
	// manifest CRD
	// istio CRDs
	remoteCrds, err := ParseRemoteCRDs([]string{
		"https://raw.githubusercontent.com/kyma-project/module-manager/main/config/crd/bases/operator.kyma-project.io_manifests.yaml", //nolint:lll
		"https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.crds.yaml",                               //nolint:lll
	})
	Expect(err).NotTo(HaveOccurred())
	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		CRDs:                  remoteCrds,
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(operatorv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(moduleManagerV1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(certManagerV1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	controlPlaneClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
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
