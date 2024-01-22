package manifest_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func setupTestEnvironment(ociTempDir, installName string) {
	It("setup OCI", func() {
		err := testutils.PushToRemoteOCIRegistry(server, manifestFilePath, installName)
		Expect(err).NotTo(HaveOccurred())
	})
	BeforeEach(
		func() {
			Expect(os.RemoveAll(filepath.Join(os.TempDir(), ociTempDir))).To(Succeed())
		},
	)
}

var _ = Describe(
	"Rendering manifest install layer", Ordered, func() {
		ociTempDir := "main-dir"
		installName := filepath.Join(ociTempDir, "installs")

		setupTestEnvironment(ociTempDir, installName)

		Context("Given a Manifest CR", func() {
			It("When Manifest CR contains a valid install OCI image specification",
				func() {
					manifest := testutils.NewTestManifest("oci")

					Eventually(testutils.WithValidInstallImageSpec(ctx, controlPlaneClient, installName,
						manifestFilePath,
						serverAddress, false, false), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Ready State", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, controlPlaneClient, shared.StateReady),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation exists", func() {
						Eventually(testutils.ExpectOCISyncRefAnnotationExists(ctx, controlPlaneClient, true),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(testutils.DeleteManifestAndVerify(ctx, controlPlaneClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
			It("When Manifest CR contains a valid install OCI image specification and enabled deploy resource",
				func() {
					manifest := testutils.NewTestManifest("oci")

					Eventually(testutils.WithValidInstallImageSpec(ctx, controlPlaneClient, installName,
						manifestFilePath,
						serverAddress, true, false), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Ready State", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, controlPlaneClient, shared.StateReady),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation exists", func() {
						Eventually(testutils.ExpectOCISyncRefAnnotationExists(ctx, controlPlaneClient, true),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(testutils.DeleteManifestAndVerify(ctx, controlPlaneClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
			It("When Manifest CR contains an invalid install OCI image specification and enabled deploy resource",
				func() {
					manifest := testutils.NewTestManifest("oci")

					Eventually(testutils.WithInvalidInstallImageSpec(ctx, controlPlaneClient, false, manifestFilePath),
						standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Error State", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, controlPlaneClient, shared.StateError),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation does not exist", func() {
						Eventually(testutils.ExpectOCISyncRefAnnotationExists(ctx, controlPlaneClient, false),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					Eventually(testutils.DeleteManifestAndVerify(ctx, controlPlaneClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
		})
	},
)

var _ = Describe(
	"Given manifest with private registry", func() {
		mainOciTempDir := "private-oci"
		installName := filepath.Join(mainOciTempDir, "crs")
		It(
			"setup remote oci Registry",
			func() {
				err := testutils.PushToRemoteOCIRegistry(server, manifestFilePath, installName)
				Expect(err).NotTo(HaveOccurred())
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
