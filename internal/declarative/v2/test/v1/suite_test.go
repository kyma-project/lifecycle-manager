package v1_test

import (
	"os"
	"path/filepath"
	"time"

	apiExtensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	testv1 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2/test/v1"

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

		testAPICRD = &apiExtensionsv1.CustomResourceDefinition{}
		testAPICRDRaw, err := os.ReadFile(
			filepath.Join(crds, "test.declarative.kyma-project.io_testapis.yaml"),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(yaml.Unmarshal(testAPICRDRaw, testAPICRD)).To(Succeed())
	},
)
