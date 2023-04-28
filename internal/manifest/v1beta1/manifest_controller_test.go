package v1beta1_test

import (
	"errors"
	"os"
	"path/filepath"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ErrManifestStateMisMatch = errors.New("ManifestState mismatch")

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
				givenCondition func(manifest *v1beta1.Manifest) error,
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
				manifest2WithInstall.Labels[v1beta1.KymaName] = manifestWithInstall.Labels[v1beta1.KymaName]
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
