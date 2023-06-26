package v1beta1_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/yaml"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ErrManifestStateMisMatch = errors.New("ManifestState mismatch")
	ErrAuthSecretErrNotFound = errors.New("auth secret error not found in manifest")
	ErrNotInErrorState       = errors.New("manifest not found in error state")
)

var _ = Describe(
	"Given manifest with OCI specs", func() {
		mainOciTempDir := "main-dir"
		installName := filepath.Join(mainOciTempDir, "installs")
		It(
			"setup OCI", func() {
				PushToRemoteOCIRegistry(installName)
			},
		)
		BeforeEach(
			func() {
				Expect(os.RemoveAll(filepath.Join(os.TempDir(), mainOciTempDir))).To(Succeed())
			},
		)
		DescribeTable(
			"Test OCI specs",
			func(
				givenCondition func(manifest *v1beta2.Manifest) error,
				expectManifestState func(manifestName string) error,
			) {
				manifest := NewTestManifest("oci")
				Eventually(givenCondition, standardTimeout, standardInterval).
					WithArguments(manifest).Should(Succeed())
				Eventually(expectManifestState, standardTimeout, standardInterval).
					WithArguments(manifest.GetName()).Should(Succeed())
				Eventually(deleteManifestAndVerify(manifest), standardTimeout, standardInterval).Should(Succeed())
			},
			Entry(
				"When Manifest CR contains a valid install OCI image specification, "+
					"expect state in ready",
				withValidInstallImageSpec(installName, false),
				expectManifestStateIn(declarative.StateReady),
			),
			Entry(
				"When Manifest CR contains a valid install OCI image specification and enabled deploy resource, "+
					"expect state in ready",
				withValidInstallImageSpec(installName, true),
				expectManifestStateIn(declarative.StateReady),
			),
			Entry(
				"When Manifest CR contains an invalid install OCI image specification, "+
					"expect state in error",
				withInvalidInstallImageSpec(false),
				expectManifestStateIn(declarative.StateError),
			),
		)
	},
)

var _ = Describe(
	"Test multiple Manifest CRs with same parent and OCI spec", func() {
		mainOciTempDir := "multiple"
		installName := filepath.Join(mainOciTempDir, "crs")
		It(
			"setup remote oci Registry",
			func() {
				PushToRemoteOCIRegistry(installName)
			},
		)
		BeforeEach(
			func() {
				Expect(os.RemoveAll(filepath.Join(os.TempDir(), mainOciTempDir))).To(Succeed())
			},
		)
		It(
			"should result in Manifest becoming Ready", func() {
				manifestWithInstall := NewTestManifest("multi-oci1")
				Eventually(withValidInstallImageSpec(installName, false), standardTimeout, standardInterval).
					WithArguments(manifestWithInstall).Should(Succeed())
				manifest2WithInstall := NewTestManifest("multi-oci2")
				// copy owner label over to the new manifest resource
				manifest2WithInstall.Labels[v1beta2.KymaName] = manifestWithInstall.Labels[v1beta2.KymaName]
				Eventually(withValidInstallImageSpec(installName, false), standardTimeout, standardInterval).
					WithArguments(manifest2WithInstall).Should(Succeed())
				Eventually(
					deleteManifestAndVerify(manifestWithInstall), standardTimeout, standardInterval,
				).Should(Succeed())
				Eventually(
					deleteManifestAndVerify(manifest2WithInstall), standardTimeout, standardInterval,
				).Should(Succeed())
			},
		)
	},
)

var _ = Describe(
	"Given manifest with private registry", func() {
		manifest := &v1beta2.Manifest{}
		manifestPath := filepath.Join("../../../pkg/test_samples/oci", "private-registry-manifest.yaml")
		manifestFile, err := os.ReadFile(manifestPath)
		Expect(err).ToNot(HaveOccurred())
		err = yaml.Unmarshal(manifestFile, manifest)
		manifest.SetNamespace(metav1.NamespaceDefault)
		manifest.SetName("private-registry-manifest")
		manifest.SetLabels(map[string]string{
			v1beta2.KymaName: string(uuid.NewUUID()),
		})
		manifest.SetResourceVersion("")
		Expect(err).ToNot(HaveOccurred())

		It("Should create Manifest", func() {
			Expect(k8sClient.Create(ctx, manifest)).To(Succeed())
		})

		It("Manifest should be in Error state with no auth secret found error message", func() {
			Eventually(func() error {
				status, err := getManifestStatus(manifest.GetName())
				if err != nil {
					return err
				}

				if status.State != declarative.StateError {
					return ErrNotInErrorState
				}
				if !strings.Contains(status.LastOperation.Operation, ocmextensions.ErrNoAuthSecretFound.Error()) {
					return ErrAuthSecretErrNotFound
				}
				return nil
			}, standardTimeout, standardInterval).
				Should(Succeed())
		})
	},
)
