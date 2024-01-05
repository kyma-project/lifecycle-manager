package manifest_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe(
	"Rendering manifest install layer", func() {
		mainOciTempDir := "main-dir"
		installName := filepath.Join(mainOciTempDir, "installs")
		It(
			"setup OCI", func() {
				testutils.PushToRemoteOCIRegistry(server, manifestFilePath, installName)
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

				Eventually(testutils.DeleteManifestAndVerify(ctx, controlPlaneClient, manifest), standardTimeout,
					standardInterval).Should(Succeed())
			},
			Entry(
				"When Manifest CR contains a valid install OCI image specification, "+
					"expect state in ready",
				testutils.WithValidInstallImageSpec(ctx, controlPlaneClient, installName, manifestFilePath,
					server.Listener.Addr().String(), false, false),
				testutils.ExpectManifestStateIn(ctx, controlPlaneClient, shared.StateReady),
				testutils.ExpectOCISyncRefAnnotationExists(ctx, controlPlaneClient, true),
			),
			Entry(
				"When Manifest CR contains a valid install OCI image specification and enabled deploy resource, "+
					"expect state in ready",
				testutils.WithValidInstallImageSpec(ctx, controlPlaneClient, installName, manifestFilePath,
					server.Listener.Addr().String(), true, false),
				testutils.ExpectManifestStateIn(ctx, controlPlaneClient, shared.StateReady),
				testutils.ExpectOCISyncRefAnnotationExists(ctx, controlPlaneClient, true),
			),
			Entry(
				"When Manifest CR contains an invalid install OCI image specification, "+
					"expect state in error",
				testutils.WithInvalidInstallImageSpec(ctx, controlPlaneClient, false, manifestFilePath),
				testutils.ExpectManifestStateIn(ctx, controlPlaneClient, shared.StateError),
				testutils.ExpectOCISyncRefAnnotationExists(ctx, controlPlaneClient, false),
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
				testutils.PushToRemoteOCIRegistry(server, manifestFilePath, installName)
			},
		)
		BeforeEach(
			func() {
				Expect(os.RemoveAll(filepath.Join(os.TempDir(), mainOciTempDir))).To(Succeed())
			},
		)

		It("Manifest should be in Error state with no auth secret found error message", func() {
			manifestWithInstall := testutils.NewTestManifest("private-oci-registry")
			Eventually(testutils.WithValidInstallImageSpec(ctx, controlPlaneClient, installName, manifestFilePath,
				server.Listener.Addr().String(), false, true),
				standardTimeout,
				standardInterval).
				WithArguments(manifestWithInstall).Should(Succeed())
			Eventually(func() string {
				status, err := testutils.GetManifestStatus(ctx, controlPlaneClient, manifestWithInstall.GetName())
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
