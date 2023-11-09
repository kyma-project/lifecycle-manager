package manifest_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	manifestctrltest "github.com/kyma-project/lifecycle-manager/tests/integration/controller/manifest/manifesttest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe(
	"Rendering manifest install layer", func() {
		mainOciTempDir := "main-dir"
		installName := filepath.Join(mainOciTempDir, "installs")
		It(
			"setup OCI", func() {
				manifestctrltest.PushToRemoteOCIRegistry(installName)
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
				expectFunctions ...func(manifestName string) error,
			) {
				manifest := testutils.NewTestManifest("oci")
				Eventually(givenCondition, standardTimeout, standardInterval).
					WithArguments(manifest).Should(Succeed())

				for _, expectFn := range expectFunctions {
					Eventually(expectFn, standardTimeout, standardInterval).
						WithArguments(manifest.GetName()).Should(Succeed())
				}

				Eventually(manifestctrltest.DeleteManifestAndVerify(manifest), standardTimeout, standardInterval).Should(Succeed())
			},
			Entry(
				"When Manifest CR contains a valid install OCI image specification, "+
					"expect state in ready",
				manifestctrltest.WithValidInstallImageSpec(installName, false, false),
				manifestctrltest.ExpectManifestStateIn(shared.StateReady),
				manifestctrltest.ExpectOCISyncRefAnnotationExists(true),
			),
			Entry(
				"When Manifest CR contains a valid install OCI image specification and enabled deploy resource, "+
					"expect state in ready",
				manifestctrltest.WithValidInstallImageSpec(installName, true, false),
				manifestctrltest.ExpectManifestStateIn(shared.StateReady),
				manifestctrltest.ExpectOCISyncRefAnnotationExists(true),
			),
			Entry(
				"When Manifest CR contains an invalid install OCI image specification, "+
					"expect state in error",
				manifestctrltest.WithInvalidInstallImageSpec(false),
				manifestctrltest.ExpectManifestStateIn(shared.StateError),
				manifestctrltest.ExpectOCISyncRefAnnotationExists(false),
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
				manifestctrltest.PushToRemoteOCIRegistry(installName)
			},
		)
		BeforeEach(
			func() {
				Expect(os.RemoveAll(filepath.Join(os.TempDir(), mainOciTempDir))).To(Succeed())
			},
		)

		It("Manifest should be in Error state with no auth secret found error message", func() {
			manifestWithInstall := testutils.NewTestManifest("private-oci-registry")
			Eventually(manifestctrltest.WithValidInstallImageSpec(installName, false, true), standardTimeout, standardInterval).
				WithArguments(manifestWithInstall).Should(Succeed())
			Eventually(func() string {
				status, err := manifestctrltest.GetManifestStatus(manifestWithInstall.GetName())
				if err != nil {
					return err.Error()
				}

				if status.State != shared.StateError {
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
