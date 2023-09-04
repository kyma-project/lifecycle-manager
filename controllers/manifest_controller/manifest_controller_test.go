package manifest_controller_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ErrManifestStateMisMatch = errors.New("ManifestState mismatch")

var _ = Describe(
	"Rendering manifest install layer", func() {
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
				manifest := testutils.NewTestManifest("oci")
				Eventually(givenCondition, standardTimeout, standardInterval).
					WithArguments(manifest).Should(Succeed())
				Eventually(expectManifestState, standardTimeout, standardInterval).
					WithArguments(manifest.GetName()).Should(Succeed())
				Eventually(deleteManifestAndVerify(manifest), standardTimeout, standardInterval).Should(Succeed())
			},
			Entry(
				"When Manifest CR contains a valid install OCI image specification, "+
					"expect state in ready",
				withValidInstallImageSpec(installName, false, false),
				expectManifestStateIn(declarative.StateReady),
			),
			Entry(
				"When Manifest CR contains a valid install OCI image specification and enabled deploy resource, "+
					"expect state in ready",
				withValidInstallImageSpec(installName, true, false),
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
	"Given manifest with private registry", func() {
		mainOciTempDir := "private-oci"
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

		It("Manifest should be in Error state with no auth secret found error message", func() {
			manifestWithInstall := testutils.NewTestManifest("private-oci-registry")
			Eventually(withValidInstallImageSpec(installName, false, true), standardTimeout, standardInterval).
				WithArguments(manifestWithInstall).Should(Succeed())
			Eventually(func() string {
				status, err := getManifestStatus(manifestWithInstall.GetName())
				if err != nil {
					return err.Error()
				}

				if status.State != declarative.StateError {
					return "manifest not in error state"
				}
				if strings.Contains(status.LastOperation.Operation, ocmextensions.ErrNoAuthSecretFound.Error()) {
					return ocmextensions.ErrNoAuthSecretFound.Error()
				}
				return status.LastOperation.Operation
			}, standardTimeout, standardInterval).
				Should(Equal(ocmextensions.ErrNoAuthSecretFound.Error()))
		})
	},
)
