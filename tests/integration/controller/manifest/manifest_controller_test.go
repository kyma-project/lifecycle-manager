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

func setupTestEnvironment(ociTempDir, installName string, mediaType v1beta2.MediaTypeMetadata) {
	It("setup OCI", func() {
		var err error
		switch mediaType {
		case v1beta2.MediaTypeDir:
			err = testutils.PushToRemoteOCIRegistry(server, manifestTarPath, installName)
		case v1beta2.MediaTypeFile:
			fallthrough
		default:
			err = testutils.PushToRemoteOCIRegistry(server, manifestFilePath, installName)
		}
		Expect(err).NotTo(HaveOccurred())
	})
	BeforeEach(
		func() {
			Expect(os.RemoveAll(filepath.Join(os.TempDir(), ociTempDir))).To(Succeed())
		},
	)
}

var _ = Describe(
	"Rendering manifest install layer from raw file", Ordered, func() {
		ociTempDir := "main-dir"
		installName := filepath.Join(ociTempDir, "installs")
		var validManifest *v1beta2.Manifest
		setupTestEnvironment(ociTempDir, installName, v1beta2.MediaTypeFile)

		Context("Given a Manifest CR", func() {
			It("When Manifest CR contains a valid install OCI image specification",
				func() {
					manifest := testutils.NewTestManifest("oci")

					Eventually(testutils.WithValidInstallImageSpecFromFile(ctx, kcpClient, installName,
						manifestFilePath,
						serverAddress, false, false), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Ready State", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, kcpClient, shared.StateReady),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation exists", func() {
						Eventually(testutils.ExpectOCISyncRefAnnotationExists(ctx, kcpClient, true),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(testutils.DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
			It("When Manifest CR contains a valid install OCI image specification and enabled deploy resource",
				func() {
					manifest := testutils.NewTestManifest("oci")

					Eventually(testutils.WithValidInstallImageSpecFromFile(ctx, kcpClient, installName,
						manifestFilePath,
						serverAddress, true, false), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					Eventually(func() error {
						var err error
						validManifest, err = testutils.GetManifestWithName(ctx, kcpClient, manifest.GetName())
						return err
					}).Should(Succeed())

					By("Then Manifest CR is in Ready State", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, kcpClient, shared.StateReady),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation exists", func() {
						Eventually(testutils.ExpectOCISyncRefAnnotationExists(ctx, kcpClient, true),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(testutils.DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
			It("When Manifest CR contains an invalid install OCI image specification and enabled deploy resource",
				func() {
					manifest := testutils.NewTestManifest("oci")

					Eventually(testutils.WithInvalidInstallImageSpec(ctx, kcpClient, false, manifestFilePath),
						standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Error State", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, kcpClient, shared.StateError),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation does not exist", func() {
						Eventually(testutils.ExpectOCISyncRefAnnotationExists(ctx, kcpClient, false),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					By("And Manifest LastOperation is updated with error message", func() {
						Eventually(testutils.ExpectManifestLastOperationMessageContains,
							standardTimeout,
							standardInterval).
							WithContext(ctx).
							WithArguments(kcpClient, manifest.GetName(),
								"failed to extract raw manifest from layer digest").
							Should(Succeed())
					})

					By("When OCI Image is corrected", func() {
						Eventually(testutils.UpdateManifestSpec, standardTimeout, standardInterval).
							WithContext(ctx).
							WithArguments(kcpClient, manifest.GetName(), validManifest.Spec).
							Should(Succeed())
					})

					By("The Manifest CR is in Ready State and lastOperation is updated correctly", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, kcpClient, shared.StateError),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).Should(Succeed())
					})
					By("And Manifest LastOperation is updated correctly", func() {
						Eventually(testutils.ExpectManifestLastOperationMessageContains,
							standardTimeout,
							standardInterval).
							WithContext(ctx).
							WithArguments(kcpClient, manifest.GetName(),
								"installation is ready and resources can be used").
							Should(Succeed())
					})

					Eventually(testutils.DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
						standardInterval).Should(Succeed())
				},
			)
		})
	},
)

var _ = Describe(
	"Rendering manifest install layer from tar", Ordered, func() {
		ociTempDir := "main-dir"
		installName := filepath.Join(ociTempDir, "installs")
		setupTestEnvironment(ociTempDir, installName, v1beta2.MediaTypeDir)

		Context("Given a Manifest CR", func() {
			It("When Manifest CR contains a valid install OCI image specification",
				func() {
					manifest := testutils.NewTestManifest("oci")

					Eventually(testutils.WithValidInstallImageSpecFromTar(ctx, kcpClient, installName,
						manifestTarPath,
						serverAddress, false, false), standardTimeout, standardInterval).
						WithArguments(manifest).
						Should(Succeed())
					By("Then Manifest CR is in Ready State", func() {
						Eventually(testutils.ExpectManifestStateIn(ctx, kcpClient, shared.StateReady),
							standardTimeout, standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					By("And OCI-Sync-Ref Annotation exists", func() {
						Eventually(testutils.ExpectOCISyncRefAnnotationExists(ctx, kcpClient, true),
							standardTimeout,
							standardInterval).
							WithArguments(manifest.GetName()).
							Should(Succeed())
					})
					Eventually(testutils.DeleteManifestAndVerify(ctx, kcpClient, manifest), standardTimeout,
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
			Eventually(testutils.WithValidInstallImageSpecFromFile(ctx, kcpClient, installName, manifestFilePath,
				server.Listener.Addr().String(), false, true),
				standardTimeout,
				standardInterval).
				WithArguments(manifestWithInstall).Should(Succeed())
			Eventually(func() string {
				status, err := testutils.GetManifestStatus(ctx, kcpClient, manifestWithInstall.GetName())
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
