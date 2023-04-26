package v1beta1_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"

	internalV1beta1 "github.com/kyma-project/lifecycle-manager/internal/manifest/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const ManifestDir = "manifest"

var ErrManifestStateMisMatch = errors.New("ManifestState mismatch")

func setHelmEnv() {
	os.Setenv(helmCacheHomeEnv, helmCacheHome)
	os.Setenv(helmCacheRepoEnv, helmCacheRepo)
	os.Setenv(helmRepoEnv, helmRepoFile)
}

var _ = Describe(
	"Given manifest with kustomize specs", func() {
		remoteKustomizeSpec := v1beta2.KustomizeSpec{
			URL:  "https://github.com/kubernetes-sigs/kustomize//examples/helloWorld/?ref=v3.3.1",
			Type: "kustomize",
		}
		remoteKustomizeSpecBytes, err := json.Marshal(remoteKustomizeSpec)
		Expect(err).ToNot(HaveOccurred())

		absoluteKustomizeLocalPath, err := filepath.Abs(kustomizeLocalPath)
		Expect(err).ToNot(HaveOccurred())
		localKustomizeSpec := v1beta2.KustomizeSpec{
			Path: absoluteKustomizeLocalPath,
			Type: "kustomize",
		}

		localKustomizeSpecBytes, err := json.Marshal(localKustomizeSpec)
		Expect(err).ToNot(HaveOccurred())

		invalidKustomizeSpec := v1beta2.KustomizeSpec{
			Path: "./invalidPath",
			Type: "kustomize",
		}
		invalidKustomizeSpecBytes, err := json.Marshal(invalidKustomizeSpec)
		Expect(err).ToNot(HaveOccurred())

		BeforeEach(
			func() {
				// reset file mode permission to 777
				Expect(os.Chmod(kustomizeLocalPath, fs.ModePerm)).To(Succeed())
			},
		)
		AfterEach(
			func() {
				Expect(os.Chmod(kustomizeLocalPath, fs.ModePerm)).To(Succeed())
			},
		)
		DescribeTable(
			"Testing Kustomize test entries",
			func(
				givenCondition func(manifest *v1beta2.Manifest) error,
				expectedManifestState func(manifestName string) error, expectedFileState func() bool,
			) {
				manifest := NewTestManifest("kust")
				Eventually(givenCondition, standardTimeout, standardInterval).
					WithArguments(manifest).Should(Succeed())
				Eventually(expectedManifestState, standardTimeout, standardInterval).
					WithArguments(manifest.GetName()).Should(Succeed())
				Eventually(expectedFileState, standardTimeout, standardInterval).Should(BeTrue())
				Eventually(deleteManifestAndVerify(manifest), standardTimeout, standardInterval).Should(Succeed())
			},
			Entry(
				"When Manifest CR contains a valid remote Kustomize specification, expect state in ready",
				addInstallSpec(remoteKustomizeSpecBytes),
				expectManifestStateIn(declarative.StateReady), skipExpect(),
			),
			Entry(
				"When Manifest CR contains a valid local Kustomize specification, expect state in ready",
				addInstallSpec(localKustomizeSpecBytes),
				expectManifestStateIn(declarative.StateReady), skipExpect(),
			),
			Entry(
				"When Manifest CR contains an invalid local Kustomize specification, expect state in error",
				addInstallSpec(invalidKustomizeSpecBytes),
				expectManifestStateIn(declarative.StateError), skipExpect(),
			),
			Entry(
				"When local Kustomize with read rights only, expect state in ready",
				addInstallSpecWithFilePermission(localKustomizeSpecBytes, false, 0o444),
				expectManifestStateIn(declarative.StateReady), skipExpect(),
			),
			Entry(
				"When local Kustomize with execute rights only, expect state in ready and file not exit",
				addInstallSpecWithFilePermission(localKustomizeSpecBytes, false, 0o555),
				expectManifestStateIn(declarative.StateReady), expectFileNotExistError(),
			),
		)
	},
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
					"expect state in ready and helmClient cache exist",
				withValidInstallImageSpec(installName, false),
				expectManifestStateIn(declarative.StateReady),
			),
			Entry(
				"When Manifest CR contains a valid install OCI image specification and enabled remote, "+
					"expect state in ready and helmClient cache exist",
				withValidInstallImageSpec(installName, true),
				expectManifestStateIn(declarative.StateReady),
			),
			Entry(
				"When Manifest CR contains valid install and CRD image specification, "+
					"expect state in ready and helmClient cache exist",
				withValidInstall(installName, true),
				expectManifestStateIn(declarative.StateReady),
			),
			Entry(
				"When Manifest CR contains an invalid install OCI image specification, "+
					"expect state in error and no helmClient cache exit",
				withInvalidInstallImageSpec(false),
				expectManifestStateIn(declarative.StateError),
			),
		)
	},
)

var _ = Describe(
	"Given Manifest CR with Helm specs", func() {
		setHelmEnv()
		validHelmChartSpec := v1beta2.HelmChartSpec{
			ChartName: "nginx-ingress",
			URL:       "https://helm.nginx.com/stable",
			Type:      "helm-chart",
		}
		validHelmChartSpecBytes, err := json.Marshal(validHelmChartSpec)
		Expect(err).ToNot(HaveOccurred())

		DescribeTable(
			"Test Helm specs",
			func(
				givenCondition func(manifest *v1beta2.Manifest) error,
				expectedBehavior func(manifestName string) error,
			) {
				manifest := NewTestManifest("helm")
				Eventually(givenCondition, standardTimeout, standardInterval).WithArguments(manifest).Should(Succeed())
				Eventually(
					expectedBehavior, standardTimeout, standardInterval,
				).WithArguments(manifest.GetName()).Should(Succeed())
				Eventually(deleteManifestAndVerify(manifest), standardTimeout, standardInterval).Should(Succeed())
			},
			Entry(
				"When manifestCR contains a valid helm repo, expect state in ready",
				addInstallSpec(validHelmChartSpecBytes), expectManifestStateIn(declarative.StateReady),
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
				validImageSpec := createOCIImageSpec(installName, server.Listener.Addr().String())
				Eventually(expectHelmClientCacheExist(true), standardTimeout, standardInterval).
					WithArguments(internalV1beta1.GenerateCacheKey(manifestWithInstall.GetLabels()[v1beta2.KymaName],
						strconv.FormatBool(manifestWithInstall.Spec.Remote), manifestWithInstall.GetNamespace())).
					Should(BeTrue())
				// this will ensure only manifest.yaml remains
				deleteHelmChartResources(validImageSpec)
				manifest2WithInstall := NewTestManifest("multi-oci2")
				// copy owner label over to the new manifest resource
				manifest2WithInstall.Labels[v1beta2.KymaName] = manifestWithInstall.Labels[v1beta2.KymaName]
				Eventually(withValidInstallImageSpec(installName, false), standardTimeout, standardInterval).
					WithArguments(manifest2WithInstall).Should(Succeed())
				// verify no new Helm resources were created
				verifyHelmResourcesDeletion(validImageSpec)
				// fresh Manifest with empty installs
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
