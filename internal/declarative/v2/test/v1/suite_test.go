package v1_test

import (
	"context"
	testv1 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2/test/v1"
	"os"
	"path/filepath"
	"time"

	apiExtensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	// this uniquely identifies a test run in the cluster with an id.
	testRunLabel = "declarative.kyma-project.io/test-run"

	standardTimeout  = 60 * time.Second
	standardInterval = 100 * time.Millisecond
)

var _ = BeforeSuite(
	func() {
		// this directory is a reference to the root directory of the project.
		root := filepath.Join("..", "..", "..", "..", "..")
		// in kubebuilder this is where CRDs are generated to with controller-gen (see make generate).
		crds := filepath.Join(root, "config", "crd", "bases")

		log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		Expect(testv1.AddToScheme(scheme.Scheme)).To(Succeed())

		testAPICRD := &apiExtensionsv1.CustomResourceDefinition{}
		testAPICRDRaw, err := os.ReadFile(
			filepath.Join(crds, "test.declarative.kyma-project.io_testapis.yaml"),
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(yaml.Unmarshal(testAPICRDRaw, testAPICRD)).To(Succeed())

		env = &envtest.Environment{
			CRDs:   []*apiExtensionsv1.CustomResourceDefinition{testAPICRD},
			Scheme: scheme.Scheme,
		}
		cfg, err = env.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())

		testClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(testClient.List(context.Background(), &testv1.TestAPIList{})).To(
			Succeed(), "Test API should be available",
		)
		Expect(err).NotTo(HaveOccurred())

		Expect(testClient.Create(context.Background(), customResourceNamespace)).To(Succeed())
	},
)

var _ = AfterSuite(
	func() {
		Expect(env.Stop()).To(Succeed())
	},
)
