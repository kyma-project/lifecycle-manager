package declarative_test

import (
	"os"
	"path/filepath"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/tests/integration"
	declarativetest "github.com/kyma-project/lifecycle-manager/tests/integration/declarative"

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
		// in kubebuilder this is where CRDs are generated to with controller-gen (see make generate).
		crds := filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")

		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		Expect(declarativetest.AddToScheme(k8sclientscheme.Scheme)).To(Succeed())

		testAPICRD = &apiextensionsv1.CustomResourceDefinition{}
		testAPICRDRaw, err := os.ReadFile(
			filepath.Join(crds, "test.declarative.kyma-project.io_testapis.yaml"),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(yaml.Unmarshal(testAPICRDRaw, testAPICRD)).To(Succeed())
	},
)
